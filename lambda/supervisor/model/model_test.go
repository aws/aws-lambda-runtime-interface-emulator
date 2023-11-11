// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"testing"
	"time"
)

// LockHard accepts deadlines encoded as RFC3339 - we enforce this with a test
func Test_KillDeadlineIsMarshalledIntoRFC3339(t *testing.T) {
	deadline, err := time.Parse(time.RFC3339, "2022-12-21T10:00:00Z")
	if err != nil {
		t.Error(err)
	}
	k := KillRequest{
		Name:     "",
		Domain:   "",
		Deadline: deadline,
	}
	bytes, err := json.Marshal(k)
	if err != nil {
		t.Error(err)
	}
	exepected := `{"name":"","domain":"","deadline":"2022-12-21T10:00:00Z"}`
	if string(bytes) != exepected {
		t.Errorf("error in marshaling `KillRequest` it does not match the expected string (Expected(%q) != Got(%q))", exepected, string(bytes))
	}
}
