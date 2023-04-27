// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/interop"
)

type RestoreBody struct {
	AwsKey            string    `json:"awskey"`
	AwsSecret         string    `json:"awssecret"`
	AwsSession        string    `json:"awssession"`
	CredentialsExpiry time.Time `json:"credentialsExpiry"`
}

func RestoreHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	restoreRequest := RestoreBody{}
	if lerr := readBodyAndUnmarshalJSON(r, &restoreRequest); lerr != nil {
		lerr.Send(w, r)
		return
	}

	restore := &interop.Restore{
		AwsKey:            restoreRequest.AwsKey,
		AwsSecret:         restoreRequest.AwsSecret,
		AwsSession:        restoreRequest.AwsSession,
		CredentialsExpiry: restoreRequest.CredentialsExpiry,
	}

	err := s.Restore(restore)

	if err != nil {
		log.Errorf("Failed to restore: %s", err)
		w.WriteHeader(http.StatusBadGateway)
	}
}
