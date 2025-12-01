// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelogs

import (
	"io"
	"time"
)

type Logger interface {
	io.Closer
	Log(op Operation, opStart time.Time, props []Property, dims []Dimension, metrics []Metric)
}

type Operation string

const (
	InitOp     Operation = "Init"
	InvokeOp   Operation = "Invoke"
	ShutdownOp Operation = "Shutdown"
)

type Tuple struct {
	Name  string
	Value string
}

type (
	Dimension = Tuple
	Property  = Tuple
)

type MetricType uint8

const (
	CounterType MetricType = iota
	TimerType
)

type Metric struct {
	Type  MetricType
	Key   string
	Value float64
	Dims  []Dimension
}

func Counter(name string, value float64, dims ...Dimension) Metric {
	return Metric{Type: CounterType, Key: name, Value: value, Dims: dims}
}

func Timer(name string, duration time.Duration, dims ...Dimension) Metric {
	return Metric{Type: TimerType, Key: name, Value: float64(duration.Microseconds()), Dims: dims}
}
