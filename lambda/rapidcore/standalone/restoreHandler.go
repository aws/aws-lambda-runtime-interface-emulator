// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/interop"
)

type RestoreBody struct {
	AwsKey               string    `json:"awskey"`
	AwsSecret            string    `json:"awssecret"`
	AwsSession           string    `json:"awssession"`
	CredentialsExpiry    time.Time `json:"credentialsExpiry"`
	RestoreHookTimeoutMs int64     `json:"restoreHookTimeoutMs"`
}

func RestoreHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	restoreRequest := RestoreBody{}
	if lerr := readBodyAndUnmarshalJSON(r, &restoreRequest); lerr != nil {
		lerr.Send(w, r)
		return
	}

	restore := &interop.Restore{
		AwsKey:               restoreRequest.AwsKey,
		AwsSecret:            restoreRequest.AwsSecret,
		AwsSession:           restoreRequest.AwsSession,
		CredentialsExpiry:    restoreRequest.CredentialsExpiry,
		RestoreHookTimeoutMs: restoreRequest.RestoreHookTimeoutMs,
	}

	restoreResult, err := s.Restore(restore)

	responseMap := make(map[string]string)

	responseMap["restoreMs"] = strconv.FormatInt(restoreResult.RestoreMs, 10)

	if err != nil {
		log.Errorf("Failed to restore: %s", err)
		responseMap["restoreError"] = err.Error()
		w.WriteHeader(http.StatusBadGateway)
	}

	responseJSON, err := json.Marshal(responseMap)

	if err != nil {
		log.Panicf("Cannot marshal the response map for RESTORE, %v", responseMap)
	}

	w.Write(responseJSON)
}
