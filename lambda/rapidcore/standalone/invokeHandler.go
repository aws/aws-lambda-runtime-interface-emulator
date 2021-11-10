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

func InvokeHandler(w http.ResponseWriter, r *http.Request, s rapidcore.InteropServer) {
	tok := s.CurrentToken()
	if tok == nil {
		log.Errorf("Attempt to call directInvoke without Reserve")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	isResyncReceivedFlag := false

	awsKey := r.Header.Get("ResyncAwsKey")
	awsSecret := r.Header.Get("ResyncAwsSecret")
	awsSession := r.Header.Get("ResyncAwsSession")

	if len(awsKey) > 0 && len(awsSecret) > 0 && len(awsSession) > 0 {
		isResyncReceivedFlag = true
	}

	invokePayload := &interop.Invoke{
		TraceID:         r.Header.Get("X-Amzn-Trace-Id"),
		LambdaSegmentID: r.Header.Get("X-Amzn-Segment-Id"),
		Payload:         r.Body,
		CorrelationID:   "invokeCorrelationID",
		DeadlineNs:      fmt.Sprintf("%d", metering.Monotime()+tok.FunctionTimeout.Nanoseconds()),
		ResyncState: interop.Resync{
			IsResyncReceived: isResyncReceivedFlag,
			AwsKey:           awsKey,
			AwsSecret:        awsSecret,
			AwsSession:       awsSession,
		},
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
