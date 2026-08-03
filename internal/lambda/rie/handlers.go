// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/core/statejson"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore/env"

	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

type Sandbox interface {
	Init(i *interop.Init, invokeTimeoutMs int64)
	AwaitInitCompletion() (time.Time, bool)
	Invoke(responseWriter http.ResponseWriter, invoke *interop.Invoke) error
}

type InteropServer interface {
	Init(i *interop.Init, invokeTimeoutMs int64) error
	AwaitInitialized() error
	FastInvoke(w http.ResponseWriter, i *interop.Invoke, direct bool) error
	Reserve(id string, traceID, lambdaSegmentID string) (*rapidcore.ReserveResponse, error)
	Reset(reason string, timeoutMs int64) (*statejson.ResetDescription, error)
	AwaitRelease() (*statejson.InternalStateDescription, error)
	Shutdown(shutdown *interop.Shutdown) *statejson.InternalStateDescription
	InternalState() (*statejson.InternalStateDescription, error)
	CurrentToken() *interop.Token
	Restore(restore *interop.Restore) error
}

var (
	initDone  bool
	initMutex sync.Mutex
)

func GetenvWithDefault(key string, defaultValue string) string {
	envValue := os.Getenv(key)

	if envValue == "" {
		return defaultValue
	}

	return envValue
}

func printEndReports(invokeId string, initDuration string, memorySize string, invokeStart time.Time, timeoutDuration time.Duration) {
	// Calcuation invoke duration
	invokeDuration := math.Min(float64(time.Now().Sub(invokeStart).Nanoseconds()),
		float64(timeoutDuration.Nanoseconds())) / float64(time.Millisecond)

	fmt.Println("END RequestId: " + invokeId)
	// We set the Max Memory Used and Memory Size to be the same (whatever it is set to) since there is
	// not a clean way to get this information from rapidcore
	fmt.Printf(
		"REPORT RequestId: %s\t"+
			initDuration+
			"Duration: %.2f ms\t"+
			"Billed Duration: %.f ms\t"+
			"Memory Size: %s MB\t"+
			"Max Memory Used: %s MB\t\n",
		invokeId, invokeDuration, math.Ceil(invokeDuration), memorySize, memorySize)
}

func startInitOnce(sandbox Sandbox, functionVersion string, timeout int64, bs interop.Bootstrap) time.Time {
	initMutex.Lock()
	defer initMutex.Unlock()

	if initDone {
		return time.Time{}
	}

	initStart := InitHandler(sandbox, functionVersion, timeout, bs)
	initDone = true
	return initStart
}

func formatInitDuration(initStart time.Time, initEnd time.Time, timeoutDuration time.Duration) string {
	if initStart.IsZero() || initEnd.IsZero() {
		return ""
	}

	initTimeMS := math.Min(float64(initEnd.Sub(initStart).Nanoseconds()),
		float64(timeoutDuration.Nanoseconds())) / float64(time.Millisecond)
	return fmt.Sprintf("Init Duration: %.2f ms\t", initTimeMS)
}

func InvokeHandler(w http.ResponseWriter, r *http.Request, sandbox Sandbox, bs interop.Bootstrap) {
	log.Debugf("invoke: -> %s %s %v", r.Method, r.URL, r.Header)
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read invoke body: %s", err)
		w.WriteHeader(500)
		return
	}

	rawClientContext, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Amz-Client-Context"))
	if err != nil {
		log.Errorf("Failed to decode X-Amz-Client-Context: %s", err)
		w.WriteHeader(500)
		return
	}

	inv := GetenvWithDefault("AWS_LAMBDA_FUNCTION_TIMEOUT", "300")
	timeoutDuration, _ := time.ParseDuration(inv + "s")
	// Default
	timeout, err := strconv.ParseInt(inv, 10, 64)
	if err != nil {
		panic(err)
	}

	functionVersion := GetenvWithDefault("AWS_LAMBDA_FUNCTION_VERSION", "$LATEST")
	memorySize := GetenvWithDefault("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", "3008")

	initStart := startInitOnce(sandbox, functionVersion, timeout, bs)

	invokeStart := time.Now()
	invokeID := r.Header.Get("X-Amzn-RequestId")
	if invokeID == "" {
		invokeID = uuid.New().String()
	}

	// Parse X-Amz-Cognito-Identity header (JSON with cognitoIdentityId and cognitoIdentityPoolId fields)
	var cognitoIdentityID, cognitoIdentityPoolID string
	if cognitoIdentityHeader := r.Header.Get("X-Amz-Cognito-Identity"); cognitoIdentityHeader != "" {
		var cognitoIdentity struct {
			CognitoIdentityID     string `json:"cognitoIdentityId"`
			CognitoIdentityPoolID string `json:"cognitoIdentityPoolId"`
		}
		if err := json.Unmarshal([]byte(cognitoIdentityHeader), &cognitoIdentity); err != nil {
			log.Errorf("Failed to parse X-Amz-Cognito-Identity header: %s", err)
			w.WriteHeader(500)
			return
		}
		cognitoIdentityID = cognitoIdentity.CognitoIdentityID
		cognitoIdentityPoolID = cognitoIdentity.CognitoIdentityPoolID
	}

	invokePayload := &interop.Invoke{
		ID:                    invokeID,
		InvokedFunctionArn:    fmt.Sprintf("arn:aws:lambda:us-east-1:012345678912:function:%s", GetenvWithDefault("AWS_LAMBDA_FUNCTION_NAME", "test_function")),
		TraceID:               r.Header.Get("X-Amzn-Trace-Id"),
		LambdaSegmentID:       r.Header.Get("X-Amzn-Segment-Id"),
		TenantID:              interop.TenantID(r.Header.Get("X-Amz-Tenant-Id")),
		Payload:               bytes.NewReader(bodyBytes),
		ClientContext:         string(rawClientContext),
		CognitoIdentityID:     cognitoIdentityID,
		CognitoIdentityPoolID: cognitoIdentityPoolID,
	}
	fmt.Println("START RequestId: " + invokePayload.ID + " Version: " + functionVersion)

	// If we write to 'w' directly and waitUntilRelease fails, we won't be able to propagate error anymore
	invokeResp := &ResponseWriterProxy{}
	if err := sandbox.Invoke(invokeResp, invokePayload); err != nil {
		switch err {

		// Reserve errors:
		case rapidcore.ErrAlreadyReserved:
			log.Errorf("Failed to reserve: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		case rapidcore.ErrInternalServerError:
			w.WriteHeader(http.StatusInternalServerError)
			return
		case rapidcore.ErrInitDoneFailed:
			w.WriteHeader(http.StatusBadGateway)
			w.Write(invokeResp.Body)
			return
		case rapidcore.ErrReserveReservationDone:
			// TODO use http.StatusBadGateway
			w.WriteHeader(http.StatusGatewayTimeout)
			return

		// Invoke errors:
		case rapidcore.ErrNotReserved:
		case rapidcore.ErrAlreadyReplied:
		case rapidcore.ErrAlreadyInvocating:
			log.Errorf("Failed to set reply stream: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		case rapidcore.ErrInvokeReservationDone:
			// TODO use http.StatusBadGateway
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		case rapidcore.ErrInvokeResponseAlreadyWritten:
			return
		// AwaitRelease errors:
		case rapidcore.ErrInvokeDoneFailed:
			w.WriteHeader(http.StatusBadGateway)
			w.Write(invokeResp.Body)
			return
		case rapidcore.ErrReleaseReservationDone:
			// TODO return sandbox status when we implement async reset handling
			// TODO use http.StatusOK
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		case rapidcore.ErrInvokeTimeout:
			initEnd, initSucceeded := sandbox.AwaitInitCompletion()
			initDuration := ""
			if initSucceeded {
				initDuration = formatInitDuration(initStart, initEnd, timeoutDuration)
			}
			printEndReports(invokePayload.ID, initDuration, memorySize, invokeStart, timeoutDuration)

			w.Write([]byte(fmt.Sprintf("Task timed out after %d.00 seconds", timeout)))
			time.Sleep(100 * time.Millisecond)
			//initDone = false
			return
		}
	}

	initEnd, initSucceeded := sandbox.AwaitInitCompletion()
	initDuration := ""
	if initSucceeded {
		initDuration = formatInitDuration(initStart, initEnd, timeoutDuration)
	}
	if !initStart.IsZero() && initEnd.After(invokeStart) {
		invokeStart = initEnd
	}
	printEndReports(invokePayload.ID, initDuration, memorySize, invokeStart, timeoutDuration)

	if invokeResp.StatusCode != 0 {
		w.WriteHeader(invokeResp.StatusCode)
	}
	w.Write(invokeResp.Body)
}

func InitHandler(sandbox Sandbox, functionVersion string, timeout int64, bs interop.Bootstrap) time.Time {
	additionalFunctionEnvironmentVariables := map[string]string{}

	// Add default Env Vars if they were not defined. This is a required otherwise 1p Python2.7, Python3.6, and
	// possibly others pre runtime API runtimes will fail. This will be overwritten if they are defined on the system.
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_LOG_GROUP_NAME"] = "/aws/lambda/Functions"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_LOG_STREAM_NAME"] = "$LATEST"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_FUNCTION_VERSION"] = "$LATEST"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_FUNCTION_MEMORY_SIZE"] = "3008"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_FUNCTION_NAME"] = "test_function"

	// Forward Env Vars from the running system (container) to what the function can view. Without this, Env Vars will
	// not be viewable when the function runs.
	for _, env := range os.Environ() {
		// Split the env into by the first "=". This will account for if the env var's value has a '=' in it
		envVar := strings.SplitN(env, "=", 2)
		additionalFunctionEnvironmentVariables[envVar[0]] = envVar[1]
	}

	initStart := time.Now()
	// pass to rapid
	sandbox.Init(&interop.Init{
		Handler:           GetenvWithDefault("AWS_LAMBDA_FUNCTION_HANDLER", os.Getenv("_HANDLER")),
		AwsKey:            os.Getenv("AWS_ACCESS_KEY_ID"),
		AwsSecret:         os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AwsSession:        os.Getenv("AWS_SESSION_TOKEN"),
		XRayDaemonAddress: "0.0.0.0:0", // TODO
		FunctionName:      GetenvWithDefault("AWS_LAMBDA_FUNCTION_NAME", "test_function"),
		FunctionVersion:   functionVersion,
		RuntimeInfo: interop.RuntimeInfo{
			ImageJSON: "{}",
			Arn:       "",
			Version:   ""},
		CustomerEnvironmentVariables: additionalFunctionEnvironmentVariables,
		SandboxType:                  interop.SandboxClassic,
		Bootstrap:                    bs,
		EnvironmentVariables:         env.NewEnvironment(),
	}, timeout*1000)
	return initStart
}
