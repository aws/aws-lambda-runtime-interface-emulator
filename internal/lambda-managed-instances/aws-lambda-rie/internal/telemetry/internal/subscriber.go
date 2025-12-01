// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Subscriber struct {
	agentName           string
	categories          map[EventCategory]struct{}
	client              Client
	curBatch            *batch
	curBatchMu          sync.Mutex
	eventCh             chan json.RawMessage
	batchSenderCh       chan batch
	bufCfg              BufferingConfig
	logsDroppedEventAPI LogsDroppedEventAPI
	flushCh             chan struct{}
}

func NewSubscriber(
	agentName string,
	categories map[EventCategory]struct{},
	bufCfg BufferingConfig,
	client Client,
	logsDroppedEventAPI LogsDroppedEventAPI,
) *Subscriber {
	s := &Subscriber{
		agentName:           agentName,
		categories:          categories,
		client:              client,
		eventCh:             make(chan json.RawMessage),
		batchSenderCh:       make(chan batch),
		bufCfg:              bufCfg,
		logsDroppedEventAPI: logsDroppedEventAPI,
		flushCh:             make(chan struct{}),
	}

	go s.batchSenderLoop()
	go s.eventConsumerLoop()
	return s
}

func (s *Subscriber) AgentName() string {
	return s.agentName
}

func (s *Subscriber) SendAsync(event json.RawMessage, category EventCategory) {
	if _, ok := s.categories[category]; !ok {
		return
	}
	s.eventCh <- event
}

func (s *Subscriber) Flush(ctx context.Context) {
	s.curBatchMu.Lock()
	b := s.curBatch
	s.curBatchMu.Unlock()
	if b == nil {

		return
	}

	s.flushCh <- struct{}{}
	select {
	case <-b.doneCh:

	case <-ctx.Done():
		slog.Warn("could not flush telemetry api subscriber", "agent_name", s.agentName)
		return
	}
}

func (s *Subscriber) eventConsumerLoop() {
	var curBatchTimer <-chan time.Time

	for {
		select {

		case <-s.flushCh:
			outBatch := s.takeCurrentBatch()
			if outBatch == nil {

				continue
			}
			s.sendCurrentBatchAsync(*outBatch)
			curBatchTimer = nil
		case <-curBatchTimer:
			slog.Debug("sending batch after batch timer expired", "agent_name", s.agentName)
			outBatch := s.takeCurrentBatch()

			s.sendCurrentBatchAsync(*outBatch)
			curBatchTimer = nil

		case event := <-s.eventCh:
			slog.Debug("processing event in eventConsumerLoop", "agent_name", s.agentName)
			s.curBatchMu.Lock()

			if s.curBatch == nil {
				s.curBatch = newBatch(s.bufCfg)
				curBatchTimer = s.curBatch.flushAt
			}

			if isFull := s.curBatch.addEvent(event); isFull {

				outBatch := s.curBatch
				s.curBatch = nil

				s.sendCurrentBatchAsync(*outBatch)
				curBatchTimer = nil

			}
			s.curBatchMu.Unlock()
		}
	}
}

func (s *Subscriber) takeCurrentBatch() *batch {
	s.curBatchMu.Lock()
	defer s.curBatchMu.Unlock()
	b := s.curBatch
	s.curBatch = nil
	return b
}

func (s *Subscriber) sendCurrentBatchAsync(batch batch) {
	select {
	case s.batchSenderCh <- batch:
		slog.Debug("sent batch to batchSenderLoop", "agent_name", s.agentName)
	default:
		slog.Warn("could not send batch to telemetry Subscriber as previous batch hasn't processed yet",
			"Subscriber", s.agentName)

		close(batch.doneCh)
		if err := s.logsDroppedEventAPI.SendPlatformLogsDropped(
			batch.sizeBytes,
			len(batch.events),
			"Some logs were dropped because the downstream consumer is slower than the logs production rate",
		); err != nil {
			slog.Error("could not send platform.logsDropped event", "err", err)
		}
	}
}

func (s *Subscriber) batchSenderLoop() {
	for outBatch := range s.batchSenderCh {
		slog.Debug("sending batch to client", "agent_name", s.agentName)
		err := s.client.send(context.Background(), outBatch)
		close(outBatch.doneCh)
		if err != nil {
			slog.Warn("could not send batch to telemetry api Subscriber",
				"Subscriber", s.agentName,
				"err", err)
			if err := s.logsDroppedEventAPI.SendPlatformLogsDropped(
				outBatch.sizeBytes,
				len(outBatch.events),
				fmt.Sprintf("could not send events: %s", err),
			); err != nil {
				slog.Error("could not send platform.logsDropped event", "err", err)
			}
		}
	}
}

type LogsDroppedEventAPI interface {
	SendPlatformLogsDropped(droppedBytes, droppedRecords int, reason string) error
}
