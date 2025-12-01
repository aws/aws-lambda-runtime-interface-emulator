// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package xray

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

const xRayNonSampled = "0"

type tracingHeader struct {
	rootId   string
	parentId string
	sampled  string

	lineage string
}

func CreateTracingData(upstreamTraceId string, tracingMode intmodel.XrayTracingMode, segmentIDGenerator func() string) (downstreamTraceId string, tracingCtx *interop.TracingCtx) {
	var segmentID string

	tracingHeader := parseTracingHeader(upstreamTraceId)

	switch tracingMode {
	case intmodel.XRayTracingModeActive:
		segmentID = segmentIDGenerator()
		downstreamTraceId = newTraceID(tracingHeader.rootId, segmentID, tracingHeader.sampled, tracingHeader.lineage)
	case intmodel.XRayTracingModePassThrough:
		downstreamTraceId = upstreamTraceId
	default:
		invariant.Violatef("Unknown tracingMode: %v", tracingMode)
	}

	tracingCtx = buildTracingCtx(tracingHeader, segmentID, upstreamTraceId)

	return downstreamTraceId, tracingCtx
}

func buildTracingCtx(t tracingHeader, segmentId, upstreamTraceId string) *interop.TracingCtx {
	if t.rootId == "" || t.sampled != rapidmodel.XRaySampled {
		return nil
	}

	return &interop.TracingCtx{
		SpanID: segmentId,
		Type:   rapidmodel.XRayTracingType,
		Value:  upstreamTraceId,
	}
}

func parseTracingHeader(trace string) tracingHeader {
	var tracingHeader tracingHeader

	keyValuePairs := strings.Split(trace, ";")
	for _, pair := range keyValuePairs {
		var key, value string
		keyValue := strings.Split(pair, "=")
		if len(keyValue) == 2 {
			key = keyValue[0]
			value = keyValue[1]
		}
		switch key {
		case "Root":
			tracingHeader.rootId = value
		case "Parent":
			tracingHeader.parentId = value
		case "Sampled":
			tracingHeader.sampled = value
		case "Lineage":
			tracingHeader.lineage = value
		}
	}
	return tracingHeader
}

func newTraceID(root, parent, sample, lineage string) string {
	if root == "" {
		return ""
	}

	parts := make([]string, 0, 4)
	parts = append(parts, "Root="+root)
	if parent != "" {
		parts = append(parts, "Parent="+parent)
	}
	if sample == "" {
		sample = xRayNonSampled
	}
	parts = append(parts, "Sampled="+sample)
	if lineage != "" {
		parts = append(parts, "Lineage="+lineage)
	}

	return strings.Join(parts, ";")
}

func GenerateSegmentID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("%08x", bytes)
}
