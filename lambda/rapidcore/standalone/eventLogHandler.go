// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.amzn.com/lambda/rapidcore/standalone/telemetry"
)

func EventLogHandler(w http.ResponseWriter, r *http.Request, eventsAPI *telemetry.StandaloneEventsAPI) {
	bytes, err := json.Marshal(eventsAPI.EventLog())
	if err != nil {
		http.Error(w, fmt.Sprintf("marshalling error: %s", err), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}
