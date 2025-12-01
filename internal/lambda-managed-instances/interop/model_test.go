// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestGetErrorResponseWithFormattedErrorMessageWithoutInvokeRequestId(t *testing.T) {
	errorType := model.ErrorRuntimeExit
	errorMessage := fmt.Errorf("Divided by 0")
	expectedMsg := fmt.Sprintf(`Error: %s`, errorMessage)
	expectedJSON := fmt.Sprintf(`{"errorType": "%s", "errorMessage": "%s"}`, string(errorType), expectedMsg)

	actual := GetErrorResponseWithFormattedErrorMessage(errorType, errorMessage, "")
	assert.Equal(t, errorType, actual.FunctionError.Type)
	assert.Equal(t, expectedMsg, actual.FunctionError.Message)
	assert.JSONEq(t, expectedJSON, string(actual.Payload))
}

func TestGetErrorResponseWithFormattedErrorMessageWithInvokeRequestId(t *testing.T) {
	errorType := model.ErrorRuntimeExit
	errorMessage := fmt.Errorf("Divided by 0")
	invokeID := "invoke-id"
	expectedMsg := fmt.Sprintf(`RequestId: %s Error: %s`, invokeID, errorMessage)
	expectedJSON := fmt.Sprintf(`{"errorType": "%s", "errorMessage": "%s"}`, string(errorType), expectedMsg)

	actual := GetErrorResponseWithFormattedErrorMessage(errorType, errorMessage, invokeID)
	assert.Equal(t, errorType, actual.FunctionError.Type)
	assert.Equal(t, expectedMsg, actual.FunctionError.Message)
	assert.JSONEq(t, expectedJSON, string(actual.Payload))
}

func TestDoneMetadataMetricsDimensionsStringWhenInvokeResponseModeIsPresent(t *testing.T) {
	testcase := []struct {
		name        string
		expectedDim string
		dim         DoneMetadataMetricsDimensions
	}{
		{
			name:        "invoke response mode is streaming",
			expectedDim: "invoke_response_mode=streaming",
			dim: DoneMetadataMetricsDimensions{
				InvokeResponseMode: InvokeResponseModeStreaming,
			},
		},
		{
			name:        "invoke response mode is buffered",
			expectedDim: "invoke_response_mode=buffered",
			dim: DoneMetadataMetricsDimensions{
				InvokeResponseMode: InvokeResponseModeBuffered,
			},
		},
	}
	for _, tt := range testcase {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedDim, tt.dim.String())
		})
	}
}

func TestDoneMetadataMetricsDimensionsStringWhenEmpty(t *testing.T) {
	dimensions := DoneMetadataMetricsDimensions{}
	assert.Equal(t, "", dimensions.String())
}

func TestGetDeadlineMs(t *testing.T) {
	invoke := Invoke{Deadline: time.Now()}

	nowCtx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	assert.Equal(t, invoke.GetDeadlineMs(context.Background()), invoke.GetDeadlineMs(nowCtx))

	notNowCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))
	defer cancel()
	assert.Less(t, invoke.GetDeadlineMs(context.Background()), invoke.GetDeadlineMs(notNowCtx))
}
