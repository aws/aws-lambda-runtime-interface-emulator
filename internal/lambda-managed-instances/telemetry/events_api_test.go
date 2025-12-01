// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestPrepareInitRuntimeDoneDataOnCustomerError(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	err := interop.ErrPlatformError
	customerErr := model.WrapErrorIntoCustomerFatalError(err, model.ErrorRuntimeInit)

	appctx.StoreFirstFatalError(appCtx, customerErr)

	stringifiedError := string(model.ErrorRuntimeInit)
	ActualInitRuntimeDoneData := prepareInitRuntimeDoneData(appCtx, customerErr, interop.LifecyclePhaseInit)
	expectedInitRuntimeDoneData := interop.InitRuntimeDoneData{
		InitializationType: "lambda-managed-instances",
		Status:             "error",
		Phase:              InitInsideInitPhase,
		ErrorType:          &stringifiedError,
	}
	assert.Equal(t, expectedInitRuntimeDoneData, ActualInitRuntimeDoneData)
}
