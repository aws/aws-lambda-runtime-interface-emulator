// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import "context"

type RuntimeNextWaiter interface {
	RuntimeNextWait(ctx context.Context) AppError
}
