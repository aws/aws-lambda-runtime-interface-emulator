// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"io"
	"net/http"

	"github.com/go-chi/chi"
	"go.amzn.com/lambda/interop"
)

const (
	InvokeIDHeader           = "Invoke-Id"
	InvokedFunctionArnHeader = "Invoked-Function-Arn"
	VersionIDHeader          = "Invoked-Function-Version"
	ReservationTokenHeader   = "Reservation-Token"
	CustomerHeadersHeader    = "Customer-Headers"
	ContentTypeHeader        = "Content-Type"

	ErrorTypeHeader = "Error-Type"

	EndOfResponseTrailer = "End-Of-Response"

	SandboxErrorType = "Error.Sandbox"
)

const (
	EndOfResponseComplete  = "Complete"
	EndOfResponseTruncated = "Truncated"
	EndOfResponseOversized = "Oversized"
)

var MaxDirectResponseSize int64 = interop.MaxPayloadSize // this is intentionally not a constant so we can configure it via CLI

func renderBadRequest(w http.ResponseWriter, r *http.Request, errorType string) {
	w.Header().Set(ErrorTypeHeader, errorType)
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set(EndOfResponseTrailer, EndOfResponseComplete)
}

// ReceiveDirectInvoke parses invoke and verifies it against Token message. Uses deadline provided by Token
// Renders BadRequest in case of error
func ReceiveDirectInvoke(w http.ResponseWriter, r *http.Request, token interop.Token) (*interop.Invoke, error) {
	w.Header().Set("Trailer", EndOfResponseTrailer)

	custHeaders := CustomerHeaders{}
	if err := custHeaders.Load(r.Header.Get(CustomerHeadersHeader)); err != nil {
		renderBadRequest(w, r, interop.ErrMalformedCustomerHeaders.Error())
		return nil, interop.ErrMalformedCustomerHeaders
	}

	inv := &interop.Invoke{
		ID:                    r.Header.Get(InvokeIDHeader),
		ReservationToken:      chi.URLParam(r, "reservationtoken"),
		InvokedFunctionArn:    r.Header.Get(InvokedFunctionArnHeader),
		VersionID:             r.Header.Get(VersionIDHeader),
		ContentType:           r.Header.Get(ContentTypeHeader),
		CognitoIdentityID:     custHeaders.CognitoIdentityID,
		CognitoIdentityPoolID: custHeaders.CognitoIdentityPoolID,
		TraceID:               token.TraceID,
		LambdaSegmentID:       token.LambdaSegmentID,
		ClientContext:         custHeaders.ClientContext,
		Payload:               r.Body,
		CorrelationID:         "invokeCorrelationID",
		DeadlineNs:            token.DeadlineNs,
	}

	if inv.ID != token.InvokeID {
		renderBadRequest(w, r, interop.ErrInvalidInvokeID.Error())
		return nil, interop.ErrInvalidInvokeID
	}

	if inv.ReservationToken != token.ReservationToken {
		renderBadRequest(w, r, interop.ErrInvalidReservationToken.Error())
		return nil, interop.ErrInvalidReservationToken
	}

	if inv.VersionID != token.VersionID {
		renderBadRequest(w, r, interop.ErrInvalidFunctionVersion.Error())
		return nil, interop.ErrInvalidFunctionVersion
	}

	w.Header().Set(VersionIDHeader, token.VersionID)
	w.Header().Set(ReservationTokenHeader, token.ReservationToken)
	w.Header().Set(InvokeIDHeader, token.InvokeID)

	return inv, nil
}

func SendDirectInvokeResponse(additionalHeaders map[string]string, payload io.Reader, w http.ResponseWriter) error {
	for k, v := range additionalHeaders {
		w.Header().Add(k, v)
	}

	n, err := io.Copy(w, io.LimitReader(payload, MaxDirectResponseSize+1)) // +1 because we do allow 10MB but not 10MB + 1 byte
	if err != nil {
		w.Header().Set(EndOfResponseTrailer, EndOfResponseTruncated)
	} else if n == MaxDirectResponseSize+1 {
		w.Header().Set(EndOfResponseTrailer, EndOfResponseOversized)
	} else {
		w.Header().Set(EndOfResponseTrailer, EndOfResponseComplete)
	}
	return err
}
