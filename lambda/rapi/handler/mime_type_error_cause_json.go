// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"fmt"

	"go.amzn.com/lambda/rapi/model"

	log "github.com/sirupsen/logrus"
)

// Validation, serialization & deserialization for
// MIME type: application/vnd.aws.lambda.error.cause+json"
type errorWithCauseRequest struct {
	ErrorMessage string          `json:"errorMessage"`
	ErrorType    string          `json:"errorType"`
	StackTrace   []string        `json:"stackTrace"`
	ErrorCause   json.RawMessage `json:"errorCause"`
}

func newErrorWithCauseRequest(requestBody []byte) (*errorWithCauseRequest, error) {
	var parsedRequest errorWithCauseRequest
	if err := json.Unmarshal(requestBody, &parsedRequest); err != nil {
		return nil, fmt.Errorf("error unmarshalling request body with error cause: %s", err)
	}

	return &parsedRequest, nil
}

func (r *errorWithCauseRequest) getInvokeErrorResponse() ([]byte, error) {
	responseBody := model.ErrorResponse{
		ErrorMessage: r.ErrorMessage,
		ErrorType:    r.ErrorType,
		StackTrace:   r.StackTrace,
	}

	filteredResponseBody, err := json.Marshal(responseBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling invocation/error response body: %s", err)
	}

	return filteredResponseBody, nil
}

func (r *errorWithCauseRequest) getValidatedXRayCause() json.RawMessage {
	if len(r.ErrorCause) == 0 {
		return nil
	}

	validErrorCauseJSON, err := model.ValidatedErrorCauseJSON(r.ErrorCause)
	if err != nil {
		log.WithError(err).Errorf("errorCause validation error, Content-Type: %s", errorWithCauseContentType)
		return nil
	}

	return validErrorCauseJSON
}
