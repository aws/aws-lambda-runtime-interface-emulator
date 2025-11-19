// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore/env"
)

type RuntimeInfo struct {
	ImageJSON string `json:"runtimeImageJSON,omitempty"`
	Arn       string `json:"runtimeArn,omitempty"`
	Version   string `json:"runtimeVersion,omitempty"`
}

// TODO: introduce suppress init flag
type InitBody struct {
	Handler         string      `json:"handler"`
	FunctionName    string      `json:"functionName"`
	FunctionVersion string      `json:"functionVersion"`
	InvokeTimeoutMs int64       `json:"invokeTimeoutMs"`
	RuntimeInfo     RuntimeInfo `json:"runtimeInfo"`
	Customer        struct {
		Environment map[string]string `json:"environment"`
	} `json:"customer"`
	AwsKey            *string   `json:"awskey"`
	AwsSecret         *string   `json:"awssecret"`
	AwsSession        *string   `json:"awssession"`
	CredentialsExpiry time.Time `json:"credentialsExpiry"`
	Throttled         bool      `json:"throttled"`
}

type InitRequest struct {
	InitBody
	ReplyChan chan Reply
}

func (c *InitBody) Validate() error {
	// Handler is optional
	if c.FunctionName == "" {
		return fmt.Errorf("functionName missing")
	}
	if c.FunctionVersion == "" {
		return fmt.Errorf("FunctionVersion missing")
	}
	if c.InvokeTimeoutMs == 0 {
		return fmt.Errorf("invokeTimeoutMs missing")
	}

	return nil
}

func InitHandler(w http.ResponseWriter, r *http.Request, sandbox InteropServer, bs interop.Bootstrap) {
	init := InitBody{}
	if lerr := readBodyAndUnmarshalJSON(r, &init); lerr != nil {
		lerr.Send(w, r)
		return
	}

	if err := init.Validate(); err != nil {
		newErrorReply(ClientInvalidRequest, err.Error()).Send(w, r)
		return
	}

	for envKey, envVal := range init.Customer.Environment {
		// We set environment variables to keep the env parsing & filtering
		// logic consistent across standalone-mode and girp-mode
		os.Setenv(envKey, envVal)
	}

	awsKey, awsSecret, awsSession := getCredentials(init)

	sandboxType := interop.SandboxClassic

	if init.Throttled {
		sandboxType = interop.SandboxPreWarmed
	}

	// pass to rapid
	sandbox.Init(&interop.Init{
		Handler:           init.Handler,
		AwsKey:            awsKey,
		AwsSecret:         awsSecret,
		AwsSession:        awsSession,
		CredentialsExpiry: init.CredentialsExpiry,
		XRayDaemonAddress: "0.0.0.0:0", // TODO
		FunctionName:      init.FunctionName,
		FunctionVersion:   init.FunctionVersion,
		RuntimeInfo: interop.RuntimeInfo{
			ImageJSON: init.RuntimeInfo.ImageJSON,
			Arn:       init.RuntimeInfo.Arn,
			Version:   init.RuntimeInfo.Version},
		CustomerEnvironmentVariables: env.CustomerEnvironmentVariables(),
		SandboxType:                  sandboxType,
		Bootstrap:                    bs,
		EnvironmentVariables:         env.NewEnvironment(),
	}, init.InvokeTimeoutMs)
}

func getCredentials(init InitBody) (string, string, string) {
	// ToDo(guvfatih): I think instead of passing and getting these credentials values via environment variables
	// we need to make StandaloneTests passing these via the Init request to be compliant with the existing protocol.
	awsKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	awsSession := os.Getenv("AWS_SESSION_TOKEN")

	if init.AwsKey != nil {
		awsKey = *init.AwsKey
	}

	if init.AwsSecret != nil {
		awsSecret = *init.AwsSecret
	}

	if init.AwsSession != nil {
		awsSession = *init.AwsSession
	}

	return awsKey, awsSecret, awsSession
}
