// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/rapidcore"
)

func ReserveHandler(w http.ResponseWriter, r *http.Request, s rapidcore.InteropServer) {
	_, internalState, err := s.Reserve("")
	if err != nil {
		switch err {
		case rapidcore.ErrAlreadyReserved:
			log.Errorf("Failed to reserve: %s", err)
			w.WriteHeader(400)
			return
		case rapidcore.ErrInitAlreadyDone:
			// init already happened before, just provide internal state and return
			InternalStateHandler(w, r, s)
			return
		case rapidcore.ErrReserveReservationDone:
			// TODO use http.StatusBadGateway
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		case rapidcore.ErrInitDoneFailed:
			w.WriteHeader(DoneFailedHTTPCode)
		case rapidcore.ErrTerminated:
			w.WriteHeader(DoneFailedHTTPCode)
			w.Write(internalState.AsJSON())
			return
		}
	}

	w.Write(internalState.AsJSON())
}
