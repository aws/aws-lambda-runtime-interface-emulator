// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"

func BuildStatusFromError(err model.AppError) ResponseStatus {
	if err == nil {
		return Success
	}

	if err.ErrorType() == model.ErrorSandboxTimedout {
		return Timeout
	}

	switch err.(type) {
	case model.CustomerError:
		return Error
	default:
		return Failure
	}
}
