// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

type Relay struct {
	mu   sync.Mutex
	subs map[string]sub

	initEventsBuffer []bufferedEvent
	disabled         bool
}

type bufferedEvent struct {
	event json.RawMessage
	cat   internal.EventCategory
}

func NewRelay() *Relay {
	return &Relay{
		subs: make(map[string]sub),
	}
}

var errSubscriberAlreadyExist = errors.New("subscriber already exists")

func (r *Relay) addSubscriber(subscriber sub) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.disabled {
		return telemetry.ErrTelemetryServiceOff
	}

	if _, ok := r.subs[subscriber.AgentName()]; ok {
		return errSubscriberAlreadyExist
	}
	r.subs[subscriber.AgentName()] = subscriber

	for _, be := range r.initEventsBuffer {
		subscriber.SendAsync(be.event, be.cat)
	}

	return nil
}

func (r *Relay) disableAddSubscriber() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disabled = true
	r.initEventsBuffer = nil
}

func (r *Relay) broadcast(record any, category internal.EventCategory, typ internal.EventType) {
	recordJSON, err := json.Marshal(record)
	invariant.Checkf(err == nil, "could not marshal record to json: %s", err)

	event := telemetry.Event{
		Time:   time.Now().UTC().Format(telemetry.TimeFormat),
		Type:   typ,
		Record: recordJSON,
	}

	b, err := json.Marshal(event)
	invariant.Checkf(err == nil, "could not marshal json telemetry event: %s", err)

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, sub := range r.subs {
		sub.SendAsync(b, category)
	}

	if !r.disabled {
		r.initEventsBuffer = append(r.initEventsBuffer, bufferedEvent{
			event: b,
			cat:   category,
		})
	}
}

func (r *Relay) flush(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(len(r.subs))
	for _, s := range r.subs {
		go func(s sub) {
			s.Flush(ctx)
			wg.Done()
		}(s)
	}
	wg.Wait()
}

type sub interface {
	AgentName() string
	SendAsync(event json.RawMessage, cat internal.EventCategory)
	Flush(ctx context.Context)
}
