// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"sync"
	"time"
)

const (
	UNBLOCKED = iota
	BLOCKED
)

var ErrCredentialsNotFound = fmt.Errorf("credentials not found for the provided token")

type Credentials struct {
	AwsKey     string    `json:"AccessKeyId"`
	AwsSecret  string    `json:"SecretAccessKey"`
	AwsSession string    `json:"Token"`
	Expiration time.Time `json:"Expiration"`
}

type CredentialsService interface {
	SetCredentials(token, awsKey, awsSecret, awsSession string, expiration time.Time)
	GetCredentials(token string) (*Credentials, error)
	UpdateCredentials(awsKey, awsSecret, awsSession string, expiration time.Time) error
}

type credentialsServiceImpl struct {
	credentials  map[string]Credentials
	contentMutex *sync.Mutex
	serviceMutex *sync.Mutex
	currentState int
}

func NewCredentialsService() CredentialsService {
	credentialsService := &credentialsServiceImpl{
		credentials:  make(map[string]Credentials),
		contentMutex: &sync.Mutex{},
		serviceMutex: &sync.Mutex{},
		currentState: UNBLOCKED,
	}

	return credentialsService
}

func (c *credentialsServiceImpl) SetCredentials(token, awsKey, awsSecret, awsSession string, expiration time.Time) {
	c.contentMutex.Lock()
	defer c.contentMutex.Unlock()

	c.credentials[token] = Credentials{
		AwsKey:     awsKey,
		AwsSecret:  awsSecret,
		AwsSession: awsSession,
		Expiration: expiration,
	}
}

func (c *credentialsServiceImpl) GetCredentials(token string) (*Credentials, error) {
	c.serviceMutex.Lock()
	defer c.serviceMutex.Unlock()

	c.contentMutex.Lock()
	defer c.contentMutex.Unlock()

	if credentials, ok := c.credentials[token]; ok {
		return &credentials, nil
	}

	return nil, ErrCredentialsNotFound
}

func (c *credentialsServiceImpl) UpdateCredentials(awsKey, awsSecret, awsSession string, expiration time.Time) error {
	mapSize := len(c.credentials)
	if mapSize != 1 {
		return fmt.Errorf("there are %d set of credentials", mapSize)
	}

	var token string
	for key := range c.credentials {
		token = key
	}

	c.SetCredentials(token, awsKey, awsSecret, awsSession, expiration)
	return nil
}
