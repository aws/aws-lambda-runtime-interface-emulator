// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

func TestRelay(t *testing.T) {
	r := NewRelay()
	r.broadcast("buffer_before_first", internal.CategoryFunction, internal.TypeFunction)
	r.flush(context.Background())

	sub1 := newMockSub(t)
	defer sub1.AssertExpectations(t)
	sub1.On("AgentName").Return("sub1")
	sub1.On("SendAsync", mock.Anything, mock.Anything).Times(4)
	sub1.On("Flush", mock.Anything).Return().Once()
	require.NoError(t, r.addSubscriber(sub1))
	require.Error(t, errSubscriberAlreadyExist, r.addSubscriber(sub1))

	r.broadcast("buffer_before_second", internal.CategoryFunction, internal.TypeFunction)
	sub2 := newMockSub(t)
	defer sub2.AssertExpectations(t)
	sub2.On("AgentName").Return("sub2")
	sub2.On("SendAsync", mock.Anything, mock.Anything).Times(4)
	sub2.On("Flush", mock.Anything).Return().Once()
	require.NoError(t, r.addSubscriber(sub2))

	r.disableAddSubscriber()
	require.Error(t, telemetry.ErrTelemetryServiceOff, r.addSubscriber(newMockSub(t)))

	r.broadcast("test_function", internal.CategoryFunction, internal.TypeFunction)
	r.broadcast("test_extension", internal.CategoryExtension, internal.TypeExtension)

	r.flush(context.Background())
}
