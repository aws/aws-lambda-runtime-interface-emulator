// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore"

	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/core/directinvoke"
)

func DirectInvokeHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	tok := s.CurrentToken()
	if tok == nil {
		log.Errorf("Attempt to call directInvoke without Reserve")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	invoke, err := directinvoke.ReceiveDirectInvoke(w, r, *tok)
	if err != nil {
		log.Errorf("direct invoke error: %s", err)
		return
	}

	if err := s.AwaitInitialized(); err != nil {
		w.WriteHeader(DoneFailedHTTPCode)
		if state, err := s.InternalState(); err == nil {
			w.Write(state.AsJSON())
		}
		return
	}

	if err := s.FastInvoke(w, invoke, true); err != nil {
		switch err {
		case rapidcore.ErrNotReserved:
		case rapidcore.ErrAlreadyReplied:
		case rapidcore.ErrAlreadyInvocating:
			log.Errorf("Failed to set reply stream: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		case rapidcore.ErrInvokeReservationDone:
			w.WriteHeader(http.StatusBadGateway)
		}
	}
}
