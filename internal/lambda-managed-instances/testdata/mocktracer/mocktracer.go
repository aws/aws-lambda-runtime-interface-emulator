// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package mocktracer

import (
	"context"
	"time"

	xray "golang.a2z.com/GoAmzn-LambdaXray"
)

var MockStartTime = time.Now().UnixNano()

var MockEndTime = time.Now().UnixNano() + 1

type MockTracer struct {
	documentsMap  map[xray.DocumentKey]xray.Document
	sentDocuments []xray.Document
}

func (m *MockTracer) Send(document xray.Document) (dk xray.DocumentKey, err error) {
	if len(document.ID) == 0 {

		document.ID = IDFor(document.Name)
	}
	m.sentDocuments = append(m.sentDocuments, document)
	return xray.DocumentKey{
		TraceID:    document.TraceID,
		DocumentID: document.ID,
	}, nil
}

func (m *MockTracer) Start(document xray.Document) (dk xray.DocumentKey, err error) {
	document.StartTime = float64(MockStartTime) / xray.TimeDenominator
	document.InProgress = true
	dk, err = m.Send(document)
	m.documentsMap[dk] = document
	return
}

func (m *MockTracer) SetOptions(dk xray.DocumentKey, documentOptions ...xray.DocumentOption) (err error) {
	document := m.documentsMap[dk]

	for _, fieldValueSetter := range documentOptions {
		fieldValueSetter(&document)
	}

	m.documentsMap[dk] = document

	return nil
}

func (m *MockTracer) End(dk xray.DocumentKey) (err error) {
	document := m.documentsMap[dk]
	document.EndTime = float64(MockEndTime) / xray.TimeDenominator
	document.InProgress = false

	m.Send(document)
	delete(m.documentsMap, dk)
	return
}

func (m *MockTracer) GetSentDocuments() []xray.Document {
	return m.sentDocuments
}

func (m *MockTracer) ResetSentDocuments() {
	m.sentDocuments = []xray.Document{}
}

func (m *MockTracer) SetDocumentMap(dm map[xray.DocumentKey]xray.Document) {
	m.documentsMap = dm
}

func (m *MockTracer) Capture(ctx context.Context, document xray.Document, criticalFunction func(context.Context) error) error {
	return nil
}

func (m *MockTracer) SetOptionsCtx(ctx context.Context, documentOptions ...xray.DocumentOption) (err error) {
	return nil
}

func NewMockTracer() xray.Tracer {
	return &MockTracer{
		documentsMap:  make(map[xray.DocumentKey]xray.Document),
		sentDocuments: []xray.Document{},
	}
}

func IDFor(name string) string {
	return name + "_SEGMID"
}
