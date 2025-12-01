// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package xray

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
)

func mockSegmentIDGenerator() string {
	return "12345678"
}

func makeExpectedTracingCtx() *interop.TracingCtx {
	return &interop.TracingCtx{
		SpanID: "",
		Type:   rapidmodel.XRayTracingType,
		Value:  "",
	}
}

func TestTracer(t *testing.T) {
	tests := []struct {
		name               string
		upstreamTraceId    string
		tracingMode        intmodel.XrayTracingMode
		segmentIDGenerator func() string

		expectedDownstreamId string
		expectedTracingCtx   *interop.TracingCtx
	}{
		{
			name:                 "Active",
			upstreamTraceId:      "Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535",
			tracingMode:          intmodel.XRayTracingModeActive,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "Root=root1;Parent=12345678;Sampled=1;Lineage=foo:1|bar:65535",
			expectedTracingCtx:   makeExpectedTracingCtx(),
		},
		{
			name:                 "Active_NoRoot",
			upstreamTraceId:      "Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535",
			tracingMode:          intmodel.XRayTracingModeActive,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "",
			expectedTracingCtx:   nil,
		},
		{
			name:                 "Active_NoParent",
			upstreamTraceId:      "Root=root1;Sampled=1;Lineage=foo:1|bar:65535",
			tracingMode:          intmodel.XRayTracingModeActive,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "Root=root1;Parent=12345678;Sampled=1;Lineage=foo:1|bar:65535",
			expectedTracingCtx:   makeExpectedTracingCtx(),
		},
		{
			name:                 "Active_NotSampled",
			upstreamTraceId:      "Root=root1;Parent=parent1;Lineage=foo:1|bar:65535",
			tracingMode:          intmodel.XRayTracingModeActive,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "Root=root1;Parent=12345678;Sampled=0;Lineage=foo:1|bar:65535",
			expectedTracingCtx:   nil,
		},
		{
			name:                 "Active_NoLineage",
			upstreamTraceId:      "Root=root1;Parent=parent1;Sampled=1",
			tracingMode:          intmodel.XRayTracingModeActive,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "Root=root1;Parent=12345678;Sampled=1",
			expectedTracingCtx:   makeExpectedTracingCtx(),
		},
		{
			name:                 "Active_UnorderedComponents",
			upstreamTraceId:      "Lineage=foo:1|bar:65535;Parent=parent1;Sampled=1;Root=root1",
			tracingMode:          intmodel.XRayTracingModeActive,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "Root=root1;Parent=12345678;Sampled=1;Lineage=foo:1|bar:65535",
			expectedTracingCtx:   makeExpectedTracingCtx(),
		},
		{
			name:                 "Active_EmptyTraceId",
			upstreamTraceId:      "",
			tracingMode:          intmodel.XRayTracingModeActive,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "",
			expectedTracingCtx:   nil,
		},
		{
			name:                 "Passthrough",
			upstreamTraceId:      "Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535",
			tracingMode:          intmodel.XRayTracingModePassThrough,
			segmentIDGenerator:   mockSegmentIDGenerator,
			expectedDownstreamId: "Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535",
			expectedTracingCtx:   makeExpectedTracingCtx(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.expectedTracingCtx != nil {
				tt.expectedTracingCtx.Value = tt.upstreamTraceId
				if tt.tracingMode == intmodel.XRayTracingModeActive {
					tt.expectedTracingCtx.SpanID = tt.segmentIDGenerator()
				}
			}

			downstreamTraceId, tracingCtx := CreateTracingData(tt.upstreamTraceId, tt.tracingMode, tt.segmentIDGenerator)

			assert.Equal(t, tt.expectedDownstreamId, downstreamTraceId)
			assert.Equal(t, tt.expectedTracingCtx, tracingCtx)

			if tt.expectedTracingCtx != nil && tt.tracingMode == intmodel.XRayTracingModeActive {

				parsedTraceId := parseTracingHeader(downstreamTraceId)
				assert.Equal(t, parsedTraceId.parentId, tracingCtx.SpanID)
			}
		})
	}
}
