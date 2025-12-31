// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

type TracingType string

const (
	// XRayTracingType represents an X-Ray Tracing object type
	XRayTracingType TracingType = "X-Amzn-Trace-Id"
)

const (
	XRaySampled    = "1"
	XRayNonSampled = "0"
)

// Tracing object returned as part of agent Invoke event
type Tracing struct {
	Type TracingType `json:"type"`
	XRayTracing
}

// XRayTracing is a type of Tracing object
type XRayTracing struct {
	Value string `json:"value"`
}

// NewXRayTracing returns a new XRayTracing object with specified value
func NewXRayTracing(value string) *Tracing {
	if len(value) == 0 {
		return nil
	}

	return &Tracing{
		XRayTracingType,
		XRayTracing{value},
	}
}
