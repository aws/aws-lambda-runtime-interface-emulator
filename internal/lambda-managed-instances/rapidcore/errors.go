// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import "errors"

var (
	ErrInitDoneFailed = errors.New("InitDoneFailed")
	ErrInitNotStarted = errors.New("InitNotStarted")
)

var (
	ErrAlreadyReplied    = errors.New("AlreadyReplied")
	ErrAlreadyInvocating = errors.New("AlreadyInvocating")
)

var (
	ErrInvokeResponseAlreadyWritten = errors.New("InvokeResponseAlreadyWritten")
	ErrInvokeDoneFailed             = errors.New("InvokeDoneFailed")
	ErrInvokeReservationDone        = errors.New("InvokeReservationDone")
)

var (
	ErrInternalServerError = errors.New("InternalServerError")
	ErrInvokeTimeout       = errors.New("InvokeTimeout")
)
