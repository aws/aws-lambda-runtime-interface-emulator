// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"context"
	"net/http"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapidcore"
)

type shutdownAPIRequest struct {
	TimeoutMs int64 `json:"timeoutMs"`
}

func ShutdownHandler(w http.ResponseWriter, r *http.Request, s rapidcore.InteropServer, shutdownFunc context.CancelFunc) {
	shutdown := shutdownAPIRequest{}
	if lerr := readBodyAndUnmarshalJSON(r, &shutdown); lerr != nil {
		lerr.Send(w, r)
		return
	}

	internalState := s.Shutdown(&interop.Shutdown{
		DeadlineNs:    metering.Monotime() + int64(shutdown.TimeoutMs*1000*1000),
		CorrelationID: "shutdownCorrelationID",
	})

	w.Write(internalState.AsJSON())

	shutdownFunc()
}
