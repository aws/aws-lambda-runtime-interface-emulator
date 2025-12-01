// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"log/slog"
	"net/http"
)

type pingHandler struct {
}

func (h *pingHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if _, err := writer.Write([]byte("pong")); err != nil {
		slog.Warn("Failed to write 'pong' response", "err", err)
		panic(err)
	}
}

func NewPingHandler() http.Handler {
	return &pingHandler{}
}
