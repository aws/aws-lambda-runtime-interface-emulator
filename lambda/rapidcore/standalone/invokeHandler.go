// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"io/ioutil"
	"net/http"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"

	log "github.com/sirupsen/logrus"
)

func InvokeHandler(w http.ResponseWriter, r *http.Request, s rapidcore.InteropServer) {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read invoke body: %s", err)
		w.WriteHeader(500)
		return
	}

	invokePayload := &interop.Invoke{
		TraceID:         r.Header.Get("X-Amzn-Trace-Id"),
		LambdaSegmentID: r.Header.Get("X-Amzn-Segment-Id"),
		Payload:         bodyBytes,
		CorrelationID:   "invokeCorrelationID",
	}

	if err := s.FastInvoke(w, invokePayload); err != nil {
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
