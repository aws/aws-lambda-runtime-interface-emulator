// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package mocktracer

import (
	"context"
	"time"

	"go.amzn.com/lambda/xray"
)

// MockStartTime is start time set in Start method
var MockStartTime = time.Now().UnixNano()

// MockEndTime is end time set in End method
var MockEndTime = time.Now().UnixNano() + 1

// MockTracer is used for unit tests
type MockTracer struct {
	documentsMap  map[xray.DocumentKey]xray.Document
	sentDocuments []xray.Document
}

// Send will add document name to sentDocuments list
func (m *MockTracer) Send(document xray.Document) (dk xray.DocumentKey, err error) {
	if len(document.ID) == 0 {
		// Give it a predictable ID that we could use in our assertions.
		document.ID = IDFor(document.Name)
	}
	m.sentDocuments = append(m.sentDocuments, document)
	return xray.DocumentKey{
		TraceID:    document.TraceID,
		DocumentID: document.ID,
	}, nil
}

// Start will save document in documentsMap
func (m *MockTracer) Start(document xray.Document) (dk xray.DocumentKey, err error) {
	document.StartTime = float64(MockStartTime) / xray.TimeDenominator
	document.InProgress = true
	dk, err = m.Send(document)
	m.documentsMap[dk] = document
	return
}

// SetOptions will set value of a field on a saved document
func (m *MockTracer) SetOptions(dk xray.DocumentKey, documentOptions ...xray.DocumentOption) (err error) {
	document := m.documentsMap[dk]

	for _, fieldValueSetter := range documentOptions {
		fieldValueSetter(&document)
	}

	m.documentsMap[dk] = document

	return nil
}

// End will delete the key-value pair in documentsMap
func (m *MockTracer) End(dk xray.DocumentKey) (err error) {
	document := m.documentsMap[dk]
	document.EndTime = float64(MockEndTime) / xray.TimeDenominator
	document.InProgress = false

	m.Send(document)
	delete(m.documentsMap, dk)
	return
}

// GetSentDocuments will return sentDocuments for unit test to verify
func (m *MockTracer) GetSentDocuments() []xray.Document {
	return m.sentDocuments
}

// ResetSentDocuments resets captured documents list to an empty list.
func (m *MockTracer) ResetSentDocuments() {
	m.sentDocuments = []xray.Document{}
}

// SetDocumentMap sets internal state.
func (m *MockTracer) SetDocumentMap(dm map[xray.DocumentKey]xray.Document) {
	m.documentsMap = dm
}

// Capture mock method for capturing segments.
func (m *MockTracer) Capture(ctx context.Context, document xray.Document, criticalFunction func(context.Context) error) error {
	return nil
}

// SetOptionsCtx contextual SetOptions.
func (m *MockTracer) SetOptionsCtx(ctx context.Context, documentOptions ...xray.DocumentOption) (err error) {
	return nil
}

// NewMockTracer is the constructor for mock tracer
func NewMockTracer() xray.Tracer {
	return &MockTracer{
		documentsMap:  make(map[xray.DocumentKey]xray.Document),
		sentDocuments: []xray.Document{},
	}
}

// IDFor constructs a predictable id for a given name.
func IDFor(name string) string {
	return name + "_SEGMID"
}
