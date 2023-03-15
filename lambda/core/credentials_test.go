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

	credentialsExpiration := time.Now().Add(15 * time.Minute)
	credentialsService.SetCredentials(Token, AwsKey, AwsSecret, AwsSession, credentialsExpiration)

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

	credentialsExpiration := time.Now().Add(15 * time.Minute)
	credentialsService.SetCredentials(Token, AwsKey, AwsSecret, AwsSession, credentialsExpiration)

	restoreCredentialsExpiration := time.Now().Add(10 * time.Hour)

	err := credentialsService.UpdateCredentials("sampleKey1", "sampleSecret1", "sampleSession1", restoreCredentialsExpiration)
	assert.NoError(t, err)

	credentials, err := credentialsService.GetCredentials(Token)

	assert.NoError(t, err)
	assert.Equal(t, "sampleKey1", credentials.AwsKey)
	assert.Equal(t, "sampleSecret1", credentials.AwsSecret)
	assert.Equal(t, "sampleSession1", credentials.AwsSession)

	nineHoursLater := time.Now().Add(9 * time.Hour)

	assert.True(t, nineHoursLater.Before(credentials.Expiration))
}

func TestUpdateCredentialsFail(t *testing.T) {
	credentialsService := NewCredentialsService()

	err := credentialsService.UpdateCredentials("unknownKey", "unknownSecret", "unknownSession", time.Now())

	assert.Error(t, err)
}
