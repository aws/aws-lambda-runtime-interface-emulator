// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	internalMocks "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testutils/mocks"
)

var responseSenderTestPayloadSize = 100

const traceIdTestHeader string = "Root=12345;Parent=67890;Sampeld=11111;Lineage=22222"

type runtimeResponseSenderMocks struct {
	ctx       context.Context
	initData  interop.MockInitStaticDataProvider
	invokeReq interop.MockInvokeRequest

	runtimeReq http.ResponseWriter
	reader     io.Reader
	writer     io.Writer
}

func createMocksAndRuntimeResponder() *runtimeResponseSenderMocks {
	mocks := runtimeResponseSenderMocks{
		ctx:        context.TODO(),
		initData:   interop.MockInitStaticDataProvider{},
		invokeReq:  interop.MockInvokeRequest{},
		runtimeReq: httptest.NewRecorder(),
		reader: &internalMocks.ReaderMock{
			WaitBeforeRead: time.Nanosecond,
			PayloadSize:    responseSenderTestPayloadSize,
		},
		writer: io.Discard,
	}

	mocks.initData.On("FunctionTimeout").Return(time.Duration(0)).Maybe()

	return &mocks
}

func checkResponseSenderExpectations(t *testing.T, mocks *runtimeResponseSenderMocks) {
	mocks.initData.AssertExpectations(t)
	mocks.invokeReq.AssertExpectations(t)
}

func buildInvokeReqMocks(invokeReq *interop.MockInvokeRequest) {
	invokeReq.On("ContentType").Return("application/json")
	invokeReq.On("InvokeID").Return("123456")
	invokeReq.On("Deadline").Return(time.Now().Add(time.Second))
	invokeReq.On("ClientContext").Return("client-context-example")
	invokeReq.On("CognitoId").Return("cognito_id_12345")
	invokeReq.On("CognitoPoolId").Return("cognito_pool_id_6789")
}

func buildInitDataMocks(initData *interop.MockInitStaticDataProvider) {
	initData.On("FunctionARN").Return("function-arn")
}

func TestSendResponseSuccess(t *testing.T) {
	t.Parallel()

	mocks := createMocksAndRuntimeResponder()
	buildInvokeReqMocks(&mocks.invokeReq)
	buildInitDataMocks(&mocks.initData)

	mocks.invokeReq.On("BodyReader").Return(mocks.reader)

	written, readerDuration, writerDuration, err := sendInvokeToRuntime(mocks.ctx, &mocks.initData, &mocks.invokeReq, mocks.runtimeReq, traceIdTestHeader)
	assert.NoError(t, err)
	assert.Equal(t, int64(responseSenderTestPayloadSize), written)
	assert.Greater(t, readerDuration, time.Duration(0))
	assert.Greater(t, writerDuration, time.Duration(0))

	checkResponseSenderExpectations(t, mocks)
}

func TestSendResponseFailure_Timeout(t *testing.T) {
	t.Parallel()

	mocks := createMocksAndRuntimeResponder()
	buildInvokeReqMocks(&mocks.invokeReq)
	buildInitDataMocks(&mocks.initData)

	mocks.invokeReq.On("BodyReader").Maybe().Return(&internalMocks.ReaderMock{
		PayloadSize:    100,
		WaitBeforeRead: time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, readerDuration, writerDuration, err := sendInvokeToRuntime(ctx, &mocks.initData, &mocks.invokeReq, mocks.runtimeReq, traceIdTestHeader)
	assert.Error(t, err)
	assert.Equal(t, model.ErrorSandboxTimedout, err.ErrorType())
	assert.Zero(t, readerDuration)
	assert.Zero(t, writerDuration)

	checkResponseSenderExpectations(t, mocks)
}

func TestSendResponseFailure_CtxCancelled(t *testing.T) {
	t.Parallel()

	mocks := createMocksAndRuntimeResponder()
	buildInvokeReqMocks(&mocks.invokeReq)
	buildInitDataMocks(&mocks.initData)

	mocks.invokeReq.On("BodyReader").Maybe().Return(&internalMocks.ReaderMock{
		PayloadSize:    100,
		WaitBeforeRead: time.Second,
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(model.NewCustomerError(model.ErrorReasonExtensionExecFailed))

	_, readerDuration, writerDuration, err := sendInvokeToRuntime(ctx, &mocks.initData, &mocks.invokeReq, mocks.runtimeReq, traceIdTestHeader)

	assert.Error(t, err)
	assert.Equal(t, model.ErrorReasonExtensionExecFailed, err.ErrorType())
	assert.Zero(t, readerDuration)
	assert.Zero(t, writerDuration)

	checkResponseSenderExpectations(t, mocks)
}
