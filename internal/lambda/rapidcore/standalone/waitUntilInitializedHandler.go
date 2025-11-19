// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore"
)

func WaitUntilInitializedHandler(w http.ResponseWriter, r *http.Request, s InteropServer) {
	err := s.AwaitInitialized()
	if err != nil {
		switch err {
		case rapidcore.ErrInitDoneFailed:
			w.WriteHeader(DoneFailedHTTPCode)
		case rapidcore.ErrInitResetReceived:
			w.WriteHeader(DoneFailedHTTPCode)
		}
	}
	w.WriteHeader(http.StatusOK)
}
