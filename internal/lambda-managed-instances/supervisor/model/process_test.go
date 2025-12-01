// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_KillDeadlineIsMarshalledIntoRFC3339(t *testing.T) {
	deadline, err := time.Parse(time.RFC3339, "2022-12-21T10:00:00Z")
	if err != nil {
		t.Error(err)
	}
	k := KillRequest{
		Name:     "",
		Deadline: deadline,
	}
	bytes, err := json.Marshal(k)
	if err != nil {
		t.Error(err)
	}
	exepected := `{"name":"","deadline":"2022-12-21T10:00:00Z"}`
	if string(bytes) != exepected {
		t.Errorf("error in marshaling `KillRequest` it does not match the expected string (Expected(%q) != Got(%q))", exepected, string(bytes))
	}
}

func TestEventToString(t *testing.T) {
	signo := int32(9)
	event := Event{
		Time: 1725043643030,
		Event: EventData{
			EvType: ProcessTerminationType,
			Name:   "runtime-1",
			Cause:  "signaled",
			Signo:  &signo,
		},
	}

	assert.Equal(t, "{Time:2024-08-30T18:47:23.030Z Event:{EvType:process_termination Name:runtime-1 Cause:signaled Signo:9 ExitStatus:<nil> Size:<nil>}}", event.String())
}

func TestEventsDerserilize(t *testing.T) {
	name := "runtime-1"
	signo := int32(9)
	exitStatus := int32(0)

	tests := map[string]struct {
		eventsJSON    string
		expectedEvent Event
	}{
		"signaled": {
			eventsJSON: `{"timestamp_millis":1686735425063,"event":{"type":"process_termination","name":"runtime-1","cause":"signaled","signo":9}}`,
			expectedEvent: Event{
				Time: 1686735425063,
				Event: EventData{
					EvType: ProcessTerminationType,
					Name:   name,
					Cause:  "signaled",
					Signo:  &signo,
				},
			},
		},
		"exited": {
			eventsJSON: `{"timestamp_millis":1686735425063,"event":{"type":"process_termination","name":"runtime-1","cause":"exited","exit_status":0}}`,
			expectedEvent: Event{
				Time: 1686735425063,
				Event: EventData{
					EvType:     ProcessTerminationType,
					Name:       name,
					Cause:      "exited",
					ExitStatus: &exitStatus,
				},
			},
		},
		"oom_killed": {
			eventsJSON: `{"timestamp_millis":1686735425063,"event":{"type":"process_termination","name":"runtime-1","cause":"oom_killed"}}`,
			expectedEvent: Event{
				Time: 1686735425063,
				Event: EventData{
					EvType: ProcessTerminationType,
					Name:   name,
					Cause:  "oom_killed",
				},
			},
		},
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			var eventStruct Event
			require.NoError(t, json.Unmarshal([]byte(data.eventsJSON), &eventStruct))
			assert.EqualValues(t, eventStruct, data.expectedEvent)
		})
	}
}

func TestOomKilledEvent(t *testing.T) {
	name := "runtime-1"
	cause := OomKilled

	ev := Event{
		Time: 1686735425063,
		Event: EventData{
			EvType: ProcessTerminationType,
			Name:   name,
			Cause:  cause,
		},
	}

	require.NotNil(t, ev.Event.ProcessTerminated())
	term := *ev.Event.ProcessTerminated()
	require.Nil(t, term.Exited())
	require.Nil(t, term.Signaled())
	require.True(t, term.OomKilled())
}

func TestSupervisorErrorDerserilize(t *testing.T) {
	tests := map[string]struct {
		eventsJSON       string
		expectedError    SupervisorError
		expectedReason   string
		expectedHookName string
		expectedSource   string
	}{
		"hook_err": {
			eventsJSON: `{
				"source": "Hook",
				"hook_name": "UnzipTask",
				"reason":"HookFailed",
				"cause": "whatever"
			}`,
			expectedError: SupervisorError{
				SourceErr:   ErrorSourceHook,
				HookNameErr: "UnzipTask",
				ReasonErr:   "HookFailed",
				CauseErr:    "whatever",
			},
			expectedReason:   "HookFailed",
			expectedHookName: "UnzipTask",
			expectedSource:   "Hook",
		},
		"client_err": {
			eventsJSON: `{"source": "Client","reason":"DomainStartFailed","cause": "whatever"}`,
			expectedError: SupervisorError{
				SourceErr: ErrorSourceClient,
				ReasonErr: "DomainStartFailed",
				CauseErr:  "whatever",
			},
			expectedReason:   "DomainStartFailed",
			expectedHookName: "",
			expectedSource:   "Client",
		},
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			var eventStruct SupervisorError
			require.NoError(t, json.Unmarshal([]byte(data.eventsJSON), &eventStruct))
			assert.EqualValues(t, eventStruct, data.expectedError)
			assert.EqualValues(t, eventStruct.Reason(), data.expectedReason)
			assert.EqualValues(t, eventStruct.HookName(), data.expectedHookName)
			assert.EqualValues(t, eventStruct.Source(), data.expectedSource)
		})
	}
}

func TestSupervisorError(t *testing.T) {
	err := SupervisorError{
		SourceErr:   ErrorSourceHook,
		ReasonErr:   "DomainStartFailed",
		CauseErr:    "whatever",
		HookNameErr: "hook1",
		ExitCodeErr: 3,
	}
	assert.EqualValues(t, err.Cause(), "whatever")
	assert.EqualValues(t, err.Error(), "DomainStartFailed")
	assert.EqualValues(t, err.ExitCode(), 3)
	assert.EqualValues(t, err.HookNameErr, "hook1")
}
