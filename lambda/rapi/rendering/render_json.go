// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rendering

import (
	"bytes"
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// RenderJSON:
// - marshals 'v' to JSON, automatically escaping HTML
// - sets the Content-Type as application/json
// - sets the HTTP response status code
// - returns an error if it occurred before writing to response
// TODO: r *http.Request is not used, remove it
func RenderJSON(status int, w http.ResponseWriter, r *http.Request, v interface{}) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.WithError(err).Warn("Error while writing response body")
	}

	return nil
}
