// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.amzn.com/lambda/rapidcore/telemetry"
)

func EventLogHandler(w http.ResponseWriter, r *http.Request, eventLog *telemetry.EventLog) {
	bytes, err := json.Marshal(eventLog)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshalling error: %s", err), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}
