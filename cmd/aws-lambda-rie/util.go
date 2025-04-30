// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"net/http"
)

type ErrorType int

const (
	ClientInvalidRequest ErrorType = iota
)

func (t ErrorType) String() string {
	switch t {
	case ClientInvalidRequest:
		return "Client.InvalidRequest"
	}
	return fmt.Sprintf("Cannot stringify standalone.ErrorType.%d", int(t))
}

type ResponseWriterProxy struct {
	Body       []byte
	StatusCode int
}

func (w *ResponseWriterProxy) Header() http.Header {
	return http.Header{}
}

func (w *ResponseWriterProxy) Write(b []byte) (int, error) {
	w.Body = b
	return 0, nil
}

func (w *ResponseWriterProxy) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}

func (w *ResponseWriterProxy) IsError() bool {
	return w.StatusCode != 0 && w.StatusCode/100 != 2
}
