// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"fmt"
	"net/http"
	"os"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"
	"go.amzn.com/lambda/rapidcore/env"
)

// TODO: introduce suppress init flag
type InitBody struct {
	Handler         string `json:"handler"`
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	InvokeTimeoutMs int64  `json:"invokeTimeoutMs"`
	Customer        struct {
		Environment map[string]string `json:"environment"`
	} `json:"customer"`
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

func InitHandler(w http.ResponseWriter, r *http.Request, sandbox rapidcore.Sandbox) {
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
	// TODO generate CorrelationID

	// pass to rapid
	sandbox.Init(&interop.Init{
		Handler:           init.Handler,
		CorrelationID:     "initCorrelationID",
		AwsKey:            os.Getenv("AWS_ACCESS_KEY_ID"),
		AwsSecret:         os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AwsSession:        os.Getenv("AWS_SESSION_TOKEN"),
		XRayDaemonAddress: "0.0.0.0:0", // TODO
		FunctionName:      init.FunctionName,
		FunctionVersion:   init.FunctionVersion,

		CustomerEnvironmentVariables: env.CustomerEnvironmentVariables(),
	}, init.InvokeTimeoutMs)

}
