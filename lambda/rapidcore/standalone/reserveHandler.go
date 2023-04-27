// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/core/directinvoke"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"
)

const (
	ReservationTokenHeader = "Reservation-Token"
	InvokeIDHeader         = "Invoke-ID"
	VersionIDHeader        = "Version-ID"
)

func tokenToHeaders(w http.ResponseWriter, token interop.Token) {
	w.Header().Set(ReservationTokenHeader, token.ReservationToken)
	w.Header().Set(directinvoke.InvokeIDHeader, token.InvokeID)
	w.Header().Set(directinvoke.VersionIDHeader, token.VersionID)
}

func ReserveHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	reserveResp, err := s.Reserve("", r.Header.Get("X-Amzn-Trace-Id"), r.Header.Get("X-Amzn-Segment-Id"))

	if err != nil {
		switch err {
		case rapidcore.ErrReserveReservationDone:
			// TODO use http.StatusBadGateway
			w.WriteHeader(http.StatusGatewayTimeout)
		default:
			log.Errorf("Failed to reserve: %s", err)
			w.WriteHeader(400)
		}
		return
	}

	tokenToHeaders(w, reserveResp.Token)
	w.Write(reserveResp.InternalState.AsJSON())
}
