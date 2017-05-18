// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"
)

var parserTests = []struct {
	traceIDIn   string
	rootIDOut   string
	parentIDOut string
	sampledOut  string
}{
	{"Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9;Sampled=1", "1-5b3cc918-939afd635f8891ba6a9e1df6", "c88d77b0aef840e9", "1"},
	{"Root=1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9", "1-5b3cc918-939afd635f8891ba6a9e1df6", "c88d77b0aef840e9", ""},
	{"1-5b3cc918-939afd635f8891ba6a9e1df6;Parent=c88d77b0aef840e9;Sampled=1", "", "c88d77b0aef840e9", "1"},
	{"Root=1-5b3cc918-939afd635f8891ba6a9e1df6", "1-5b3cc918-939afd635f8891ba6a9e1df6", "", ""},
}

func TestParseTraceID(t *testing.T) {
	for _, tt := range parserTests {
		t.Run(tt.traceIDIn, func(t *testing.T) {
			rootID, parentID, sampled := ParseTraceID(tt.traceIDIn)
			if rootID != tt.rootIDOut {
				t.Errorf("got %q, wanted %q", rootID, tt.rootIDOut)
			}
			if parentID != tt.parentIDOut {
				t.Errorf("got %q, wanted %q", rootID, tt.parentIDOut)
			}
			if sampled != tt.sampledOut {
				t.Errorf("got %q, wanted %q", sampled, tt.sampledOut)
			}
		})
	}
}
