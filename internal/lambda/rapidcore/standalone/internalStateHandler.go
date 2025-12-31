// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"
)

func InternalStateHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	state, err := s.InternalState()
	if err != nil {
		http.Error(w, "internal state callback not set", http.StatusInternalServerError)
		return
	}

	w.Write(state.AsJSON())
}
