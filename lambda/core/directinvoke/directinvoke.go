// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"go.amzn.com/lambda/core/bandwidthlimiter"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"

	log "github.com/sirupsen/logrus"
)

const (
	InvokeIDHeader                   = "Invoke-Id"
	InvokedFunctionArnHeader         = "Invoked-Function-Arn"
	VersionIDHeader                  = "Invoked-Function-Version"
	ReservationTokenHeader           = "Reservation-Token"
	CustomerHeadersHeader            = "Customer-Headers"
	ContentTypeHeader                = "Content-Type"
	MaxPayloadSizeHeader             = "MaxPayloadSize"
	ResponseBandwidthRateHeader      = "ResponseBandwidthRate"
	ResponseBandwidthBurstSizeHeader = "ResponseBandwidthBurstSize"
	FunctionResponseModeHeader       = "Lambda-Runtime-Function-Response-Mode"

	ErrorTypeHeader = "Error-Type"

	EndOfResponseTrailer     = "End-Of-Response"
	FunctionErrorTypeTrailer = "Lambda-Runtime-Function-Error-Type"
	FunctionErrorBodyTrailer = "Lambda-Runtime-Function-Error-Body"
)

const (
	EndOfResponseComplete  = "Complete"
	EndOfResponseTruncated = "Truncated"
	EndOfResponseOversized = "Oversized"
)

var ResetReasonMap = map[string]fatalerror.ErrorType{
	"failure": fatalerror.SandboxFailure,
	"timeout": fatalerror.SandboxTimeout,
}

var MaxDirectResponseSize int64 = interop.MaxPayloadSize // this is intentionally not a constant so we can configure it via CLI
var ResponseBandwidthRate int64 = interop.ResponseBandwidthRate
var ResponseBandwidthBurstSize int64 = interop.ResponseBandwidthBurstSize

func renderBadRequest(w http.ResponseWriter, r *http.Request, errorType string) {
	w.Header().Set(ErrorTypeHeader, errorType)
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set(EndOfResponseTrailer, EndOfResponseComplete)
}

func renderInternalServerError(w http.ResponseWriter, errorType string) {
	w.Header().Set(ErrorTypeHeader, errorType)
	w.WriteHeader(http.StatusInternalServerError)
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

	now := metering.Monotime()

	MaxDirectResponseSize = interop.MaxPayloadSize
	if maxPayloadSize := r.Header.Get(MaxPayloadSizeHeader); maxPayloadSize != "" {
		if n, err := strconv.ParseInt(maxPayloadSize, 10, 64); err == nil && n >= -1 {
			MaxDirectResponseSize = n
		} else {
			log.Error("MaxPayloadSize header is not a valid number")
			renderBadRequest(w, r, interop.ErrInvalidMaxPayloadSize.Error())
			return nil, interop.ErrInvalidMaxPayloadSize
		}
	}

	if MaxDirectResponseSize == -1 {
		w.Header().Add("Trailer", FunctionErrorTypeTrailer)
		w.Header().Add("Trailer", FunctionErrorBodyTrailer)

		ResponseBandwidthRate = interop.ResponseBandwidthRate
		if responseBandwidthRate := r.Header.Get(ResponseBandwidthRateHeader); responseBandwidthRate != "" {
			if n, err := strconv.ParseInt(responseBandwidthRate, 10, 64); err == nil &&
				interop.MinResponseBandwidthRate <= n && n <= interop.MaxResponseBandwidthRate {
				ResponseBandwidthRate = n
			} else {
				log.Error("ResponseBandwidthRate header is not a valid number or is out of the allowed range")
				renderBadRequest(w, r, interop.ErrInvalidResponseBandwidthRate.Error())
				return nil, interop.ErrInvalidResponseBandwidthRate
			}
		}

		ResponseBandwidthBurstSize = interop.ResponseBandwidthBurstSize
		if responseBandwidthBurstSize := r.Header.Get(ResponseBandwidthBurstSizeHeader); responseBandwidthBurstSize != "" {
			if n, err := strconv.ParseInt(responseBandwidthBurstSize, 10, 64); err == nil &&
				interop.MinResponseBandwidthBurstSize <= n && n <= interop.MaxResponseBandwidthBurstSize {
				ResponseBandwidthBurstSize = n
			} else {
				log.Error("ResponseBandwidthBurstSize header is not a valid number or is out of the allowed range")
				renderBadRequest(w, r, interop.ErrInvalidResponseBandwidthBurstSize.Error())
				return nil, interop.ErrInvalidResponseBandwidthBurstSize
			}
		}
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
		DeadlineNs:            fmt.Sprintf("%d", now+token.FunctionTimeout.Nanoseconds()),
		NeedDebugLogs:         token.NeedDebugLogs,
		InvokeReceivedTime:    now,
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

	if now > token.InvackDeadlineNs {
		renderBadRequest(w, r, interop.ErrReservationExpired.Error())
		return nil, interop.ErrReservationExpired
	}

	w.Header().Set(VersionIDHeader, token.VersionID)
	w.Header().Set(ReservationTokenHeader, token.ReservationToken)
	w.Header().Set(InvokeIDHeader, token.InvokeID)

	return inv, nil
}

type CopyDoneResult struct {
	Metrics *interop.InvokeResponseMetrics
	Error   error
}

func getErrorTypeFromResetReason(resetReason string) fatalerror.ErrorType {
	errorTypeTrailer, ok := ResetReasonMap[resetReason]
	if !ok {
		errorTypeTrailer = fatalerror.Unknown
	}
	return errorTypeTrailer
}

func isErrorResponse(additionalHeaders map[string]string) (isErrorResponse bool) {
	_, isErrorResponse = additionalHeaders[ErrorTypeHeader]
	return
}

func isStreamingInvoke() bool {
	return MaxDirectResponseSize == -1
}

func asyncPayloadCopy(w http.ResponseWriter, payload io.Reader) (copyDone chan CopyDoneResult, cancel context.CancelFunc, err error) {
	copyDone = make(chan CopyDoneResult)
	streamedResponseWriter, cancel, err := NewStreamedResponseWriter(w)
	if err != nil {
		return nil, nil, &interop.ErrInternalPlatformError{}
	}
	go func() { // copy payload in a separate go routine
		_, copyError := bandwidthlimiter.BandwidthLimitingCopy(streamedResponseWriter, payload)
		if copyError != nil {
			w.Header().Set(EndOfResponseTrailer, EndOfResponseTruncated)
		} else {
			w.Header().Set(EndOfResponseTrailer, EndOfResponseComplete)
		}
		copyDoneResult := CopyDoneResult{
			Metrics: streamedResponseWriter.GetMetrics(),
			Error:   copyError,
		}
		copyDone <- copyDoneResult
		cancel() // free resources
	}()
	return
}

func sendStreamingInvokeResponse(payload io.Reader, trailers http.Header, w http.ResponseWriter,
	interruptedResponseChan chan *interop.Reset, sendResponseChan chan *interop.InvokeResponseMetrics,
	request *interop.CancellableRequest, runtimeCalledResponse bool) (err error) {
	/* In case of /response, we copy the payload and, once copied, we attach:
	 * 1) 'Lambda-Runtime-Function-Error-Type'
	 * 2) 'Lambda-Runtime-Function-Error-Body'
	 * trailers. */
	copyDone, cancel, err := asyncPayloadCopy(w, payload)
	if err != nil {
		renderInternalServerError(w, err.Error())
		return err
	}

	var errorTypeTrailer string
	var errorBodyTrailer string
	var copyDoneResult CopyDoneResult
	select {
	case copyDoneResult = <-copyDone: // copy finished
		errorTypeTrailer = trailers.Get(FunctionErrorTypeTrailer)
		errorBodyTrailer = trailers.Get(FunctionErrorBodyTrailer)
		if copyDoneResult.Error != nil && errorTypeTrailer == "" { // truncated payload, error type not known
			errorTypeTrailer = string(fatalerror.TruncatedResponse)
		}
	case reset := <-interruptedResponseChan: // reset initiated
		cancel()
		if request != nil {
			// In case of reset:
			// * to interrupt copying when runtime called /response (a potential stuck on Body.Read() operation),
			//   we close the underlying connection using .Close() method on the request object
			// * for /error case, the whole body is already read in /error handler, so we don't need additional handling
			//   when sending streaming invoke error response
			connErr := request.Cancel()
			if connErr != nil {
				log.Warnf("Failed to close underlying connection: %s", connErr)
			}
		} else {
			log.Warn("Cannot close underlying connection. Request object is nil")
		}
		copyDoneResult = <-copyDone
		reset.InvokeResponseMetrics = copyDoneResult.Metrics
		interruptedResponseChan <- nil
		errorTypeTrailer = string(getErrorTypeFromResetReason(reset.Reason))
	}
	w.Header().Set(FunctionErrorTypeTrailer, errorTypeTrailer)
	w.Header().Set(FunctionErrorBodyTrailer, errorBodyTrailer)

	copyDoneResult.Metrics.RuntimeCalledResponse = runtimeCalledResponse
	sendResponseChan <- copyDoneResult.Metrics

	if copyDoneResult.Error != nil {
		log.Errorf("Error while streaming response payload: %s", copyDoneResult.Error)
		err = &interop.ErrTruncatedResponse{}
	}
	return
}

func sendStreamingInvokeErrorResponse(payload io.Reader, w http.ResponseWriter,
	interruptedResponseChan chan *interop.Reset, sendResponseChan chan *interop.InvokeResponseMetrics, runtimeCalledResponse bool) (err error) {

	copyDone, cancel, err := asyncPayloadCopy(w, payload)
	if err != nil {
		renderInternalServerError(w, err.Error())
		return err
	}

	var copyDoneResult CopyDoneResult
	select {
	case copyDoneResult = <-copyDone: // copy finished
	case reset := <-interruptedResponseChan: // reset initiated
		cancel()
		copyDoneResult = <-copyDone
		reset.InvokeResponseMetrics = copyDoneResult.Metrics
		interruptedResponseChan <- nil
	}

	copyDoneResult.Metrics.RuntimeCalledResponse = runtimeCalledResponse
	sendResponseChan <- copyDoneResult.Metrics

	if copyDoneResult.Error != nil {
		log.Errorf("Error while streaming error response payload: %s", copyDoneResult.Error)
		err = &interop.ErrTruncatedResponse{}
	}
	return
}

// parseFunctionResponseMode fetches the mode from the header
// If the header is absent, it returns buffered mode.
func parseFunctionResponseMode(w http.ResponseWriter) (interop.FunctionResponseMode, error) {
	headerValue := w.Header().Get(FunctionResponseModeHeader)
	// the header is optional, so it's ok to be absent
	if headerValue == "" {
		return interop.FunctionResponseModeBuffered, nil
	}

	return interop.ConvertToFunctionResponseMode(headerValue)
}

func sendPayloadLimitedResponse(payload io.Reader, trailers http.Header, w http.ResponseWriter, sendResponseChan chan *interop.InvokeResponseMetrics, runtimeCalledResponse bool) (err error) {
	functionResponseMode, err := parseFunctionResponseMode(w)
	if err != nil {
		return err
	}

	// non-streaming invoke request but runtime is streaming: predefine Trailer headers
	if functionResponseMode == interop.FunctionResponseModeStreaming {
		w.Header().Add("Trailer", FunctionErrorTypeTrailer)
		w.Header().Add("Trailer", FunctionErrorBodyTrailer)
	}

	startReadingResponseMonoTimeMs := metering.Monotime()
	written, err := io.Copy(w, io.LimitReader(payload, MaxDirectResponseSize+1)) // +1 because we do allow 10MB but not 10MB + 1 byte

	// non-streaming invoke request but runtime is streaming: set response trailers
	if functionResponseMode == interop.FunctionResponseModeStreaming {
		w.Header().Set(FunctionErrorTypeTrailer, trailers.Get(FunctionErrorTypeTrailer))
		w.Header().Set(FunctionErrorBodyTrailer, trailers.Get(FunctionErrorBodyTrailer))
	}

	if err != nil {
		w.Header().Set(EndOfResponseTrailer, EndOfResponseTruncated)
		err = &interop.ErrTruncatedResponse{}
	} else if MaxDirectResponseSize != -1 && written == MaxDirectResponseSize+1 {
		w.Header().Set(EndOfResponseTrailer, EndOfResponseOversized)
		err = &interop.ErrorResponseTooLargeDI{
			ErrorResponseTooLarge: interop.ErrorResponseTooLarge{
				ResponseSize:    int(written),
				MaxResponseSize: int(MaxDirectResponseSize),
			},
		}
	} else {
		w.Header().Set(EndOfResponseTrailer, EndOfResponseComplete)
	}

	sendResponseChan <- &interop.InvokeResponseMetrics{
		ProducedBytes:                   int64(written),
		StartReadingResponseMonoTimeMs:  startReadingResponseMonoTimeMs,
		FinishReadingResponseMonoTimeMs: metering.Monotime(),
		TimeShapedNs:                    int64(-1),
		OutboundThroughputBps:           int64(-1),
		// FIXME:
		// We should use InvokeResponseMode here, because only when it's streaming we're interested
		// on it. If the invoke is buffered, we don't generate streaming metrics, even if the
		// function response mode is streaming.
		FunctionResponseMode:  interop.FunctionResponseModeBuffered,
		RuntimeCalledResponse: runtimeCalledResponse,
	}
	return
}

func SendDirectInvokeResponse(additionalHeaders map[string]string, payload io.Reader, trailers http.Header,
	w http.ResponseWriter, interruptedResponseChan chan *interop.Reset,
	sendResponseChan chan *interop.InvokeResponseMetrics, request *interop.CancellableRequest, runtimeCalledResponse bool) error {

	for k, v := range additionalHeaders {
		w.Header().Add(k, v)
	}

	if isStreamingInvoke() { // unlimited payload; response streaming mode
		if isErrorResponse(additionalHeaders) { // send streamed error response when runtime called /error
			return sendStreamingInvokeErrorResponse(payload, w, interruptedResponseChan, sendResponseChan, runtimeCalledResponse)
		}
		// send streamed response when runtime called /response
		return sendStreamingInvokeResponse(payload, trailers, w, interruptedResponseChan, sendResponseChan, request, runtimeCalledResponse)
	}

	return sendPayloadLimitedResponse(payload, trailers, w, sendResponseChan, runtimeCalledResponse)
}
