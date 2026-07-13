// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"fmt"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

const (
	ReserveSuccessMetric       = "ReserveSuccess"
	ReserveFailedMetric        = "ReserveFailed"
	ReserveParseDurationMetric = "ReserveParseDuration"
	ReserveLogicDurationMetric = "ReserveLogicDuration"
)

func ReserveServiceLog(logger servicelogs.Logger, opStart time.Time, invokeID string, appErr model.AppError, parseDuration, reserveDuration time.Duration) {
	props := []servicelogs.Property{
		{Name: "invoke_id", Value: invokeID},
	}

	var clientErrCnt, nonCustomerErrCnt float64
	success := float64(1)
	failed := float64(0)

	if appErr != nil {
		success = 0
		failed = 1

		switch appErr.(type) {
		case model.ClientError:
			clientErrCnt = 1
		default:
			nonCustomerErrCnt = 1
		}
	}

	metrics := []servicelogs.Metric{
		servicelogs.Counter(ReserveSuccessMetric, success),
		servicelogs.Counter(ReserveFailedMetric, failed),
		servicelogs.Counter(interop.ClientErrorMetric, clientErrCnt),
		servicelogs.Counter(interop.NonCustomerErrorMetric, nonCustomerErrCnt),
		servicelogs.Timer(ReserveParseDurationMetric, parseDuration),
	}

	if reserveDuration > 0 {
		metrics = append(metrics, servicelogs.Timer(ReserveLogicDurationMetric, reserveDuration))
	}

	if appErr != nil {
		metrics = append(metrics,
			servicelogs.Counter(fmt.Sprintf(interop.ClientErrorReasonTemplate, appErr.ErrorType()), 1.0),
		)
	}

	logger.Log(servicelogs.ReserveOp, opStart, props, nil, metrics)
}
