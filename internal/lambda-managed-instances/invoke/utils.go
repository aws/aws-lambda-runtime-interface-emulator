// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func BuildInvokeAppError(err error, invokeTimeout time.Duration) model.AppError {
	var appError model.AppError

	switch {
	case errors.As(err, &appError):
		return appError
	case errors.Is(err, context.DeadlineExceeded):
		return model.NewCustomerError(
			model.ErrorSandboxTimedout,
			model.WithCause(err),
			model.WithErrorMessage(fmt.Sprintf("Task timed out after %.2f seconds", invokeTimeout.Seconds())),
		)
	default:
		return model.NewPlatformError(err, "BuildInvokeAppError doesn't know this error")
	}
}
