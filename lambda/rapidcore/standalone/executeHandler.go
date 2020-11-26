// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"
)

func Execute(w http.ResponseWriter, r *http.Request, sandbox rapidcore.Sandbox) {
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

	// If we write to 'w' directly and waitUntilRelease fails, we won't be able to propagate error anymore
	invokeResp := &ResponseWriterProxy{}
	if err := sandbox.Invoke(invokeResp, invokePayload); err != nil {
		switch err {
		// Reserve errors:
		case rapidcore.ErrAlreadyReserved:
			log.Errorf("Failed to reserve: %s", err)
			w.WriteHeader(400)
		case rapidcore.ErrInternalServerError:
			w.WriteHeader(http.StatusInternalServerError)

		// Invoke errors:
		case rapidcore.ErrNotReserved, rapidcore.ErrAlreadyReplied, rapidcore.ErrAlreadyInvocating:
			log.Errorf("Failed to set reply stream: %s", err)
			w.WriteHeader(400)

		case rapidcore.ErrInvokeResponseAlreadyWritten:
			return
		case rapidcore.ErrInvokeTimeout:
			w.WriteHeader(http.StatusGatewayTimeout)

		// DONE failures:
		case rapidcore.ErrTerminated, rapidcore.ErrInitDoneFailed, rapidcore.ErrInvokeDoneFailed:
			w.WriteHeader(DoneFailedHTTPCode)
			w.Write(invokeResp.Body)
			return
		// Reservation canceled errors
		case rapidcore.ErrReserveReservationDone, rapidcore.ErrInvokeReservationDone, rapidcore.ErrReleaseReservationDone:
			w.WriteHeader(http.StatusGatewayTimeout)
		}

		return
	}

	if invokeResp.StatusCode != 0 {
		w.WriteHeader(invokeResp.StatusCode)
	}
	w.Write(invokeResp.Body)
}
