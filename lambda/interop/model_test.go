// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"fmt"
	"testing"

	"go.amzn.com/lambda/fatalerror"

	"github.com/stretchr/testify/assert"
)

func TestMergeSubscriptionMetrics(t *testing.T) {
	logsAPIMetrics := map[string]int{
		"server_error": 1,
		"client_error": 2,
	}

	telemetryAPIMetrics := map[string]int{
		"server_error": 1,
		"success":      5,
	}

	metrics := MergeSubscriptionMetrics(logsAPIMetrics, telemetryAPIMetrics)
	assert.Equal(t, 5, metrics["success"])
	assert.Equal(t, 2, metrics["server_error"])
	assert.Equal(t, 2, metrics["client_error"])
}

func TestGetErrorResponseWithFormattedErrorMessageWithoutInvokeRequestId(t *testing.T) {
	errorType := fatalerror.RuntimeExit
	errorMessage := fmt.Errorf("Divided by 0")
	expectedMsg := fmt.Sprintf(`Error: %s`, errorMessage)
	expectedJSON := fmt.Sprintf(`{"errorType": "%s", "errorMessage": "%s"}`, string(errorType), expectedMsg)

	actual := GetErrorResponseWithFormattedErrorMessage(errorType, errorMessage, "")
	assert.Equal(t, errorType, actual.FunctionError.Type)
	assert.Equal(t, expectedMsg, actual.FunctionError.Message)
	assert.JSONEq(t, expectedJSON, string(actual.Payload))
}

func TestGetErrorResponseWithFormattedErrorMessageWithInvokeRequestId(t *testing.T) {
	errorType := fatalerror.RuntimeExit
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
	dimensions := DoneMetadataMetricsDimensions{
		InvokeResponseMode: InvokeResponseModeStreaming,
	}
	assert.Equal(t, "invoke_response_mode=streaming", dimensions.String())
}
func TestDoneMetadataMetricsDimensionsStringWhenEmpty(t *testing.T) {
	dimensions := DoneMetadataMetricsDimensions{}
	assert.Equal(t, "", dimensions.String())
}
