// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"fmt"
	"net/http"
	"strconv"

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

	restoreDurationHeader := r.Header.Get("restore-duration")
	restoreStartHeader := r.Header.Get("restore-start-time")

	var restoreDurationNs int64 = 0
	var restoreStartTimeMonotime int64 = 0
	if restoreDurationHeader != "" && restoreStartHeader != "" {
		var err1, err2 error
		restoreDurationNs, err1 = strconv.ParseInt(restoreDurationHeader, 10, 64)
		restoreStartTimeMonotime, err2 = strconv.ParseInt(restoreStartHeader, 10, 64)
		if err1 != nil || err2 != nil {
			log.Errorf("Failed to parse 'restore-duration' from '%s' and/or 'restore-start-time' from '%s'", restoreDurationHeader, restoreStartHeader)
			restoreDurationNs = 0
			restoreStartTimeMonotime = 0
		}
	}

	invokePayload := &interop.Invoke{
		TraceID:                  r.Header.Get("X-Amzn-Trace-Id"),
		LambdaSegmentID:          r.Header.Get("X-Amzn-Segment-Id"),
		Payload:                  r.Body,
		DeadlineNs:               fmt.Sprintf("%d", metering.Monotime()+tok.FunctionTimeout.Nanoseconds()),
		InvokeReceivedTime:       metering.Monotime(),
		RestoreDurationNs:        restoreDurationNs,
		RestoreStartTimeMonotime: restoreStartTimeMonotime,
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
