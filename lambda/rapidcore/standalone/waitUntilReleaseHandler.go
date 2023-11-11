// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"

	"go.amzn.com/lambda/rapidcore"
)

func WaitUntilReleaseHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	releaseAwait, err := s.AwaitRelease()
	if err != nil {
		switch err {
		case rapidcore.ErrInvokeDoneFailed:
			w.WriteHeader(http.StatusBadGateway)
		case rapidcore.ErrReleaseReservationDone:
			// TODO return sandbox status when we implement async reset handling
			// TODO use http.StatusOK
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		case rapidcore.ErrInitDoneFailed:
			w.WriteHeader(DoneFailedHTTPCode)
			w.Write(releaseAwait.AsJSON())
			return
		}
	}

	w.Write(releaseAwait.AsJSON())
}
