// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

type pingHandler struct {
	//
}

func (h *pingHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if _, err := writer.Write([]byte("pong")); err != nil {
		log.WithError(err).Fatal("Failed to write 'pong' response")
	}
}

// NewPingHandler returns a new instance of http handler
// for serving /ping.
func NewPingHandler() http.Handler {
	return &pingHandler{}
}
