// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSubscriber(t *testing.T) {
	t.Parallel()

	client := NewMockClient(t)
	logsDroppedEventAPI := NewMockLogsDroppedEventAPI(t)

	agentName := fmt.Sprintf("test-name-%d", rand.Uint32())
	sub := NewSubscriber(agentName, map[EventCategory]struct{}{CategoryPlatform: {}}, BufferingConfig{MaxItems: 2, MaxBytes: math.MaxInt, Timeout: math.MaxInt}, client, logsDroppedEventAPI)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, agentName, sub.AgentName())

	sub.Flush(context.Background())
	client.AssertExpectations(t)

	event := json.RawMessage("data")
	sub.SendAsync(event, CategoryFunction)
	sub.SendAsync(event, CategoryExtension)
	sub.Flush(context.Background())
	client.AssertExpectations(t)

	sub.SendAsync(event, CategoryPlatform)
	client.AssertExpectations(t)

	client.On("send", mock.Anything, mock.Anything).Return(nil)
	sub.SendAsync(event, CategoryPlatform)

	require.Eventually(t, func() bool {
		return client.AssertNumberOfCalls(t, "send", 1)
	}, time.Second, 10*time.Millisecond)

	sub.SendAsync(event, CategoryPlatform)
	assert.Eventually(
		t,
		func() bool {

			sub.Flush(context.Background())
			return client.AssertNumberOfCalls(t, "send", 2)
		},
		time.Second,
		10*time.Millisecond,
	)
}
