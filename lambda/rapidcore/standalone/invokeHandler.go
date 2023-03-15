// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"fmt"
	"net/http"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapidcore"

	log "github.com/sirupsen/logrus"
)

func InvokeHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	tok := s.CurrentToken()
	if tok == nil {
		log.Errorf("Attempt to call directInvoke without Reserve")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	invokePayload := &interop.Invoke{
		TraceID:            r.Header.Get("X-Amzn-Trace-Id"),
		LambdaSegmentID:    r.Header.Get("X-Amzn-Segment-Id"),
		Payload:            r.Body,
		DeadlineNs:         fmt.Sprintf("%d", metering.Monotime()+tok.FunctionTimeout.Nanoseconds()),
		InvokeReceivedTime: metering.Monotime(),
	}

	if err := s.AwaitInitialized(); err != nil {
		w.WriteHeader(DoneFailedHTTPCode)
		if state, err := s.InternalState(); err == nil {
			w.Write(state.AsJSON())
		}
		return
	}

	if err := s.FastInvoke(w, invokePayload, false); err != nil {
		switch err {
		case rapidcore.ErrNotReserved:
		case rapidcore.ErrAlreadyReplied:
		case rapidcore.ErrAlreadyInvocating:
			log.Errorf("Failed to set reply stream: %s", err)
			w.WriteHeader(400)
			return
		case rapidcore.ErrInvokeReservationDone:
			// TODO use http.StatusBadGateway
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	}
}
