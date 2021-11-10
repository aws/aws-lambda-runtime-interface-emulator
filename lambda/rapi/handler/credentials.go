// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"go.amzn.com/lambda/core"
)

type credentialsHandler struct {
	credentialsService core.CredentialsService
}

func (h *credentialsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	token := request.Header.Get("Authorization")

	credentials, err := h.credentialsService.GetCredentials(token)

	if err != nil {
		errorMsg := "cannot get credentials for the provided token"
		log.WithError(err).Error(errorMsg)
		http.Error(writer, errorMsg, http.StatusNotFound)
		return
	}

	jsonResponse, _ := json.Marshal(*credentials)
	fmt.Fprint(writer, string(jsonResponse))
}

func NewCredentialsHandler(credentialsService core.CredentialsService) http.Handler {
	return &credentialsHandler{
		credentialsService: credentialsService,
	}
}
