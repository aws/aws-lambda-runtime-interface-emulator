// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.amzn.com/lambda/rapi/model"
)

var BigString = strings.Repeat("a", 255)

var parserTests = []struct {
	tracingHeaderIn string
	rootIDOut       string
	parentIDOut     string
	sampledOut      string
	lineageOut      string
}{
	{"Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9;Sampled=1", "1-5b3cc918-939afd635f8891ba6a9e1df6", "c88d77b0aef840e9", "1", ""},
	{"Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9", "1-5b3cc918-939afd635f8891ba6a9e1df6", "c88d77b0aef840e9", "", ""},
	{"1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9;Sampled=1", "", "c88d77b0aef840e9", "1", ""},
	{"Root=1-5b3cc918-939afd635f8891ba6a9e1df6", "1-5b3cc918-939afd635f8891ba6a9e1df6", "", "", ""},
	{"", "", "", "", ""},
	{"abc;;", "", "", "", ""},
	{"abc", "", "", "", ""},
	{"abc;asd", "", "", "", ""},
	{"abc=as;asd=as", "", "", "", ""},
	{"Root=abc", "abc", "", "", ""},
	{"Root=abc;Parent=zxc;Sampled=1", "abc", "zxc", "1", ""},
	{"Root=root;Parent=par", "root", "par", "", ""},
	{"Root=root;Par", "root", "", "", ""},
	{"Root=", "", "", "", ""},
	{";Root=root;;", "root", "", "", ""},
	{"Root=root;Parent=parent;", "root", "parent", "", ""},
	{"Root=;Parent=parent;Sampled=1", "", "parent", "1", ""},
	{"Root=abc;Parent=zxc;Sampled=1;Lineage", "abc", "zxc", "1", ""},
	{"Root=abc;Parent=zxc;Sampled=1;Lineage=", "abc", "zxc", "1", ""},
	{"Root=abc;Parent=zxc;Sampled=1;Lineage=foo:1|bar:65535", "abc", "zxc", "1", "foo:1|bar:65535"},
	{"Root=abc;Parent=zxc;Lineage=foo:1|bar:65535;Sampled=1", "abc", "zxc", "1", "foo:1|bar:65535"},
	{fmt.Sprintf("Root=%s;Parent=%s;Sampled=1;Lineage=%s", BigString, BigString, BigString), BigString, BigString, "1", BigString},
}

func TestParseTracingHeader(t *testing.T) {
	for _, tt := range parserTests {
		t.Run(tt.tracingHeaderIn, func(t *testing.T) {
			rootID, parentID, sampled, lineage := ParseTracingHeader(tt.tracingHeaderIn)
			if rootID != tt.rootIDOut {
				t.Errorf("Parsing %q got %q, wanted %q", tt.tracingHeaderIn, rootID, tt.rootIDOut)
			}
			if parentID != tt.parentIDOut {
				t.Errorf("Parsing %q got %q, wanted %q", tt.tracingHeaderIn, parentID, tt.parentIDOut)
			}
			if sampled != tt.sampledOut {
				t.Errorf("Parsing %q got %q, wanted %q", tt.tracingHeaderIn, sampled, tt.sampledOut)
			}
			if lineage != tt.lineageOut {
				t.Errorf("Parsing %q got %q, wanted %q", tt.tracingHeaderIn, lineage, tt.lineageOut)
			}
			if lineage != tt.lineageOut {
				t.Errorf("got %q, wanted %q", lineage, tt.lineageOut)
			}
		})
	}
}

func TestBuildFullTraceID(t *testing.T) {
	specs := map[string]struct {
		root            string
		parent          string
		sample          string
		expectedTraceID string
	}{
		"all non-empty components, sampled": {
			root:            "1-5b3cc918-939afd635f8891ba6a9e1df6",
			parent:          "c88d77b0aef840e9",
			sample:          model.XRaySampled,
			expectedTraceID: "Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9;Sampled=1",
		},
		"all non-empty components, non-sampled": {
			root:            "1-5b3cc918-939afd635f8891ba6a9e1df6",
			parent:          "c88d77b0aef840e9",
			sample:          model.XRayNonSampled,
			expectedTraceID: "Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9;Sampled=0",
		},
		"root is non-empty, parent and sample are empty": {
			root:            "1-5b3cc918-939afd635f8891ba6a9e1df6",
			expectedTraceID: "Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Sampled=0",
		},
		"root is empty": {
			parent:          "c88d77b0aef840e9",
			expectedTraceID: "",
		},
		"sample is empty": {
			root:            "1-5b3cc918-939afd635f8891ba6a9e1df6",
			parent:          "c88d77b0aef840e9",
			expectedTraceID: "Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9;Sampled=0",
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			actual := BuildFullTraceID(spec.root, spec.parent, spec.sample)
			if actual != spec.expectedTraceID {
				t.Errorf("got %q, wanted %q", actual, spec.expectedTraceID)
			}
		})
	}
}

func TestTracerDoesntSwallowErrorsFromCriticalFunctions(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		tracer        Tracer
		expectedError error
	}{
		{
			name:          "NoOpTracer-success",
			tracer:        &NoOpTracer{},
			expectedError: nil,
		},
		{
			name:          "NoOpTracer-fail",
			tracer:        &NoOpTracer{},
			expectedError: fmt.Errorf("invoke error"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			criticalFunction := func(ctx context.Context) error {
				return test.expectedError
			}

			if err := test.tracer.CaptureInvokeSegment(ctx, criticalFunction); err != test.expectedError {
				t.Errorf("CaptureInvokeSegment failed; expected: '%v', but got: '%v'", test.expectedError, err)
			}
			if err := test.tracer.CaptureInitSubsegment(ctx, criticalFunction); err != test.expectedError {
				t.Errorf("CaptureInitSubsegment failed; expected: '%v', but got: '%v'", test.expectedError, err)
			}
			if err := test.tracer.CaptureInvokeSubsegment(ctx, criticalFunction); err != test.expectedError {
				t.Errorf("CaptureInvokeSubsegment failed; expected: '%v', but got: '%v'", test.expectedError, err)
			}
			if err := test.tracer.CaptureOverheadSubsegment(ctx, criticalFunction); err != test.expectedError {
				t.Errorf("CaptureOverheadSubsegment failed; expected: '%v', but got: '%v'", test.expectedError, err)
			}
		})
	}
}
