// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/rapi/model"
)

const (
	DoneFailedHTTPCode = 502
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

func readBodyAndUnmarshalJSON(r *http.Request, dst interface{}) *ErrorReply {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return newErrorReply(ClientInvalidRequest, fmt.Sprintf("Failed to read full body: %s", err))
	}

	if err = json.Unmarshal(bodyBytes, dst); err != nil {
		return newErrorReply(ClientInvalidRequest, fmt.Sprintf("Invalid json %s: %s", string(bodyBytes), err))
	}

	return nil
}

type ErrorReply struct {
	model.ErrorResponse
}

type RuntimeErrorReply struct {
	Payload []byte
}

func (e *RuntimeErrorReply) Send(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(500)
	w.Write(e.Payload)
}

func newErrorReply(errType ErrorType, errMsg string) *ErrorReply {
	return &ErrorReply{ErrorResponse: model.ErrorResponse{ErrorType: errType.String(), ErrorMessage: errMsg}}
}

func (e *ErrorReply) Send(w http.ResponseWriter, r *http.Request) {
	http.Error(w, e.ErrorType, 400)
	bodyJSON, err := json.Marshal(*e)
	if err != nil {
		http.Error(w, "Invalid format", 500)
		log.Errorf("Failed to Marshal(%#v): %s", e, err)
	} else {
		w.Write(bodyJSON)
	}
}

type SuccessReply struct {
	Body []byte
}

func (s *SuccessReply) Send(w http.ResponseWriter, r *http.Request) {
	w.Write(s.Body)
}

type FailureReply struct {
	Body []byte
}

func (s *FailureReply) Send(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(DoneFailedHTTPCode)
	w.Write(s.Body)
}

type Reply interface {
	Send(http.ResponseWriter, *http.Request)
}
