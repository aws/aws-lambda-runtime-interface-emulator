// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type Responder struct {
	invokeReq      interop.InvokeRequest
	body           []byte
	responseWriter http.ResponseWriter
}

func NewResponder(invokeReq interop.InvokeRequest, responseWriter http.ResponseWriter) *Responder {
	return &Responder{
		invokeReq:      invokeReq,
		responseWriter: responseWriter,
	}
}

func (s *Responder) SendRuntimeResponseHeaders(_ interop.InitStaticDataProvider, _, _ string) {

}

func (s *Responder) SendRuntimeResponseBody(_ context.Context, runtimeResp invoke.RuntimeResponseRequest, _ time.Duration) invoke.SendResponseBodyResult {
	runtimeBodyReader := io.LimitReader(runtimeResp.BodyReader(), s.invokeReq.MaxPayloadSize()+1)
	b, err := io.ReadAll(runtimeBodyReader)
	if err != nil {
		return invoke.SendResponseBodyResult{
			Err: model.NewCustomerError(model.ErrorRuntimeTruncatedResponse, model.WithCause(err)),
		}
	}
	if len(b) > int(s.invokeReq.MaxPayloadSize()) {
		errorResponseTooLarge := interop.ErrorResponseTooLarge{
			ResponseSize:    len(b),
			MaxResponseSize: int(s.invokeReq.MaxPayloadSize()),
		}
		return invoke.SendResponseBodyResult{
			Err: model.NewCustomerError(model.ErrorFunctionOversizedResponse, model.WithCause(&errorResponseTooLarge)),
		}
	}
	s.body = b

	return invoke.SendResponseBodyResult{}
}

func (s *Responder) SendRuntimeResponseTrailers(request invoke.RuntimeResponseRequest) {
	trailerError := request.TrailerError()
	if trailerError != nil {
		s.SendErrorTrailers(trailerError, "")
		return
	}
	s.responseWriter.Header().Set(invoke.СontentTypeHeader, request.ContentType())
	s.responseWriter.Header().Set(invoke.RuntimeResponseModeHeader, request.ResponseMode())
	if _, err := s.responseWriter.Write(s.body); err != nil {
		slog.Error("could not write invoke response", "err", err)
	}
}

func (s *Responder) SendError(err invoke.ErrorForInvoker, _ interop.InitStaticDataProvider) {
	s.SendErrorTrailers(err, "")
}

func (s *Responder) SendErrorTrailers(err invoke.ErrorForInvoker, _ invoke.InvokeBodyResponseStatus) {
	s.responseWriter.Header().Set("Error-Type", err.ErrorType().String())

	s.responseWriter.WriteHeader(err.ReturnCode())
	if _, err := s.responseWriter.Write([]byte(err.ErrorDetails())); err != nil {
		slog.Error("could not write invoke error response", "err", err)
	}
}

func (s *Responder) ErrorPayloadSizeBytes() int {
	return 0
}
