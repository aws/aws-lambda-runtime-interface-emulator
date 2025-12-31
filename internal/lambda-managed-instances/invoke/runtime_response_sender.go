// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core/directinvoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	rapiModel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

const (
	RuntimeContentTypeHeader     = "Content-Type"
	RuntimeRequestIdHeader       = "Lambda-Runtime-Aws-Request-Id"
	RuntimeDeadlineHeader        = "Lambda-Runtime-Deadline-Ms"
	RuntimeFunctionArnHeader     = "Lambda-Runtime-Invoked-Function-Arn"
	RuntimeTraceIdHeader         = "Lambda-Runtime-Trace-Id"
	RuntimeClientContextHeader   = "Lambda-Runtime-Client-Context"
	RuntimeCognitoIdentifyHeader = "Lambda-Runtime-Cognito-Identity"
)

func sendInvokeToRuntime(ctx context.Context, initData interop.InitStaticDataProvider, invokeReq interop.InvokeRequest, runtimeReq http.ResponseWriter, traceId string) (int64, time.Duration, time.Duration, model.AppError) {
	runtimeReq.Header().Set(RuntimeContentTypeHeader, invokeReq.ContentType())
	runtimeReq.Header().Set(RuntimeRequestIdHeader, invokeReq.InvokeID())
	runtimeReq.Header().Set(RuntimeDeadlineHeader, strconv.FormatInt(invokeReq.Deadline().UnixMilli(), 10))
	runtimeReq.Header().Set(RuntimeFunctionArnHeader, initData.FunctionARN())
	runtimeReq.Header().Set(RuntimeTraceIdHeader, traceId)
	runtimeReq.Header().Set(RuntimeClientContextHeader, invokeReq.ClientContext())
	runtimeReq.Header().Set(RuntimeCognitoIdentifyHeader, buildCognitoIdentifyHeader(invokeReq))
	runtimeReq.WriteHeader(http.StatusOK)

	timedReader := &utils.TimedReader{
		Reader: invokeReq.BodyReader(),
		Name:   "request",
		Ctx:    ctx,
	}
	timedWriter := &utils.TimedWriter{
		Writer: directinvoke.NewCancellableWriter(ctx, runtimeReq),
		Name:   "request",
		Ctx:    ctx,
	}

	resChan := make(chan error)
	var written int64
	go func() {
		var err error
		written, err = utils.CopyWithPool(timedWriter, timedReader)
		resChan <- err
	}()

	select {
	case <-ctx.Done():

		return 0, 0, 0, BuildInvokeAppError(context.Cause(ctx), initData.FunctionTimeout())
	case err := <-resChan:
		if err != nil {
			return 0, timedWriter.TotalDuration, timedWriter.TotalDuration, model.NewCustomerError(model.ErrorRuntimeUnknown, model.WithCause(err))
		}
		return written, timedReader.TotalDuration, timedWriter.TotalDuration, nil
	}
}

func buildCognitoIdentifyHeader(invokeReq interop.InvokeRequest) string {
	cognitoIdentityJSON := ""
	if len(invokeReq.CognitoId()) != 0 || len(invokeReq.CognitoPoolId()) != 0 {
		cognitoJSON, err := json.Marshal(rapiModel.CognitoIdentity{
			CognitoIdentityID:     invokeReq.CognitoId(),
			CognitoIdentityPoolID: invokeReq.CognitoPoolId(),
		})

		if err != nil {
			slog.Error("Marshal cognitoIdentity returns error", "err", err)
			return ""
		}

		cognitoIdentityJSON = string(cognitoJSON)
	}

	return cognitoIdentityJSON
}
