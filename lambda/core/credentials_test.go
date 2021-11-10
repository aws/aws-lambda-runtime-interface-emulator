// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	Token      string = "sampleToken"
	AwsKey     string = "sampleKey"
	AwsSecret  string = "sampleSecret"
	AwsSession string = "sampleSession"
)

func TestGetSetCredentialsHappy(t *testing.T) {
	credentialsService := NewCredentialsService()

	credentialsService.SetCredentials(Token, AwsKey, AwsSecret, AwsSession)

	credentials, err := credentialsService.GetCredentials(Token)

	assert.NoError(t, err)
	assert.Equal(t, AwsKey, credentials.AwsKey)
	assert.Equal(t, AwsSecret, credentials.AwsSecret)
	assert.Equal(t, AwsSession, credentials.AwsSession)
}

func TestGetCredentialsFail(t *testing.T) {
	credentialsService := NewCredentialsService()

	_, err := credentialsService.GetCredentials("unknownToken")

	assert.Error(t, err)
}

func TestUpdateCredentialsHappy(t *testing.T) {
	credentialsService := NewCredentialsService()

	credentialsService.SetCredentials(Token, AwsKey, AwsSecret, AwsSession)
	err := credentialsService.UpdateCredentials("sampleKey1", "sampleSecret1", "sampleSession1")
	assert.NoError(t, err)

	credentials, err := credentialsService.GetCredentials(Token)

	assert.NoError(t, err)
	assert.Equal(t, "sampleKey1", credentials.AwsKey)
	assert.Equal(t, "sampleSecret1", credentials.AwsSecret)
	assert.Equal(t, "sampleSession1", credentials.AwsSession)
}

func TestUpdateCredentialsFail(t *testing.T) {
	credentialsService := NewCredentialsService()

	err := credentialsService.UpdateCredentials("unknownKey", "unknownSecret", "unknownSession")

	assert.Error(t, err)
}

func TestUpdateCredentialsOfBlockedService(t *testing.T) {
	credentialsService := NewCredentialsService()
	credentialsService.BlockService()
	credentialsService.SetCredentials(Token, AwsKey, AwsSecret, AwsSession)
	err := credentialsService.UpdateCredentials("sampleKey1", "sampleSecret1", "sampleSession1")
	assert.NoError(t, err)
}

func TestConsecutiveBlockService(t *testing.T) {
	credentialsService := NewCredentialsService()

	timeout := time.After(1 * time.Second)
	done := make(chan bool)

	go func() {
		for i := 0; i < 10; i++ {
			credentialsService.BlockService()
		}
		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("BlockService should not block the calling thread.")
	case <-done:
	}
}

// unlocking a mutex twice causes panic
// the assertion here is basically not having panic
func TestConsecutiveUnblockService(t *testing.T) {
	credentialsService := NewCredentialsService()

	credentialsService.UnblockService()
	credentialsService.UnblockService()
}
