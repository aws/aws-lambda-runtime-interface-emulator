// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import "errors"

var ErrInitAlreadyDone = errors.New("InitAlreadyDone")
var ErrInitDoneFailed = errors.New("InitDoneFailed")
var ErrInitError = errors.New("InitError")

var ErrNotReserved = errors.New("NotReserved")
var ErrAlreadyReserved = errors.New("AlreadyReserved")
var ErrAlreadyReplied = errors.New("AlreadyReplied")
var ErrAlreadyInvocating = errors.New("AlreadyInvocating")
var ErrReserveReservationDone = errors.New("ReserveReservationDone")

var ErrInvokeResponseAlreadyWritten = errors.New("InvokeResponseAlreadyWritten")
var ErrInvokeDoneFailed = errors.New("InvokeDoneFailed")
var ErrInvokeReservationDone = errors.New("InvokeReservationDone")

var ErrReleaseReservationDone = errors.New("ReleaseReservationDone")

var ErrInternalServerError = errors.New("InternalServerError")
var ErrInvokeTimeout = errors.New("InvokeTimeout")

var ErrTerminated = errors.New("SandboxTerminated") // sent to signal a process exit
