// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type cognitoIdentity struct {
	CognitoIdentityID     string `json:"cognitoIdentityId"`
	CognitoIdentityPoolID string `json:"cognitoIdentityPoolId"`
}

type rieInvokeRequest struct {
	request *http.Request
	writer  http.ResponseWriter

	contentType                string
	maxPayloadSize             int64
	responseBandwidthRate      int64
	responseBandwidthBurstSize int64
	invokeID                   interop.InvokeID
	deadline                   time.Time
	traceId                    string
	cognitoIdentityId          string
	cognitoIdentityPoolId      string
	clientContext              string
	responseMode               string

	functionVersionID string
}

func NewRieInvokeRequest(request *http.Request, writer http.ResponseWriter) (*rieInvokeRequest, model.AppError) {

	contentType := request.Header.Get(invoke.СontentTypeHeader)
	if contentType == "" {
		contentType = "application/json"
	}

	invokeID := request.Header.Get(RequestIdHeader)
	if invokeID == "" {
		invokeID = uuid.New().String()
	}

	clientContext := ""
	if encodedClientContext := request.Header.Get(ClientContextHeader); encodedClientContext != "" {
		decodedClientContext, err := base64.StdEncoding.DecodeString(encodedClientContext)
		if err != nil {
			slog.Warn("Failed to decode X-Amz-Client-Context header", "err", err)
			return nil, model.NewClientError(
				fmt.Errorf("X-Amz-Client-Context must be a valid base64 encoded string: %w", err),
				model.ErrorSeverityInvalid,
				model.ErrorMalformedRequest,
			)
		}
		clientContext = string(decodedClientContext)
	}

	var cognitoIdentityId, cognitoIdentityPoolId string
	if cognitoIdentityHeader := request.Header.Get(CognitoIdentityHeader); cognitoIdentityHeader != "" {
		var cognito cognitoIdentity
		if err := json.Unmarshal([]byte(cognitoIdentityHeader), &cognito); err != nil {
			slog.Warn("Failed to parse X-Amz-Cognito-Identity header", "err", err)
			return nil, model.NewClientError(
				fmt.Errorf("X-Amz-Cognito-Identity must be a valid JSON string: %w", err),
				model.ErrorSeverityInvalid,
				model.ErrorMalformedRequest,
			)
		}
		cognitoIdentityId = cognito.CognitoIdentityID
		cognitoIdentityPoolId = cognito.CognitoIdentityPoolID
	}

	req := &rieInvokeRequest{
		request:                    request,
		writer:                     writer,
		invokeID:                   invokeID,
		contentType:                contentType,
		maxPayloadSize:             6*1024*1024 + 100,
		responseBandwidthRate:      2 * 1024 * 1024,
		responseBandwidthBurstSize: 6 * 1024 * 1024,
		traceId:                    request.Header.Get(invoke.TraceIdHeader),
		cognitoIdentityId:          cognitoIdentityId,
		cognitoIdentityPoolId:      cognitoIdentityPoolId,
		clientContext:              clientContext,
		responseMode:               request.Header.Get(invoke.ResponseModeHeader),
	}

	return req, nil
}

func (r *rieInvokeRequest) ContentType() string {
	return r.contentType
}

func (r *rieInvokeRequest) InvokeID() interop.InvokeID {
	return r.invokeID
}

func (r *rieInvokeRequest) Deadline() time.Time {
	return r.deadline
}

func (r *rieInvokeRequest) TraceId() string {
	return r.traceId
}

func (r *rieInvokeRequest) ClientContext() string {
	return r.clientContext
}

func (r *rieInvokeRequest) CognitoId() string {
	return r.cognitoIdentityId
}

func (r *rieInvokeRequest) CognitoPoolId() string {
	return r.cognitoIdentityPoolId
}

func (r *rieInvokeRequest) ResponseBandwidthRate() int64 {
	return r.responseBandwidthRate
}

func (r *rieInvokeRequest) ResponseBandwidthBurstRate() int64 {
	return r.responseBandwidthBurstSize
}

func (r *rieInvokeRequest) MaxPayloadSize() int64 {
	return r.maxPayloadSize
}

func (r *rieInvokeRequest) BodyReader() io.Reader {
	return r.request.Body
}

func (r *rieInvokeRequest) ResponseWriter() http.ResponseWriter {
	return r.writer
}

func (r *rieInvokeRequest) SetResponseHeader(key string, val string) {
	r.writer.Header().Set(key, val)
}

func (r *rieInvokeRequest) AddResponseHeader(key string, val string) {
	r.writer.Header().Add(key, val)
}

func (r *rieInvokeRequest) WriteResponseHeaders(status int) {
	r.writer.WriteHeader(status)
}

func (r *rieInvokeRequest) ResponseMode() string {
	return r.responseMode
}

func (r *rieInvokeRequest) UpdateFromInitData(initData interop.InitStaticDataProvider) model.AppError {
	if initData == nil {
		return model.NewClientError(errors.New("sandbox is not initialized"), model.ErrorSeverityError, model.ErrorInitIncomplete)
	}

	r.deadline = time.Now().Add(time.Duration(initData.FunctionTimeout()) * time.Millisecond)

	if r.functionVersionID != initData.FunctionVersionID() {
		return model.NewClientError(nil, model.ErrorSeverityInvalid, model.ErrorInvalidFunctionVersion)
	}

	return nil
}

func (r *rieInvokeRequest) FunctionVersionID() string {
	return r.functionVersionID
}
