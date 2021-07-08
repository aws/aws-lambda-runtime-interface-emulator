// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"

	"go.amzn.com/lambda/rapidcore"
)

type resetAPIRequest struct {
	Reason    string `json:"reason"`
	TimeoutMs int64  `json:"timeoutMs"`
}

func ResetHandler(w http.ResponseWriter, r *http.Request, s rapidcore.InteropServer) {
	reset := resetAPIRequest{}
	if lerr := readBodyAndUnmarshalJSON(r, &reset); lerr != nil {
		lerr.Send(w, r)
		return
	}

	resetDescription, err := s.Reset(reset.Reason, reset.TimeoutMs)
	if err != nil {
		(&FailureReply{}).Send(w, r)
		return
	}

	w.Write(resetDescription.AsJSON())
}
