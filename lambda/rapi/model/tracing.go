// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

const (
	// XRayTracingType represents an X-Ray Tracing object type
	XRayTracingType = "X-Amzn-Trace-Id"
)

// Tracing object returned as part of agent Invoke event
type Tracing struct {
	Type string `json:"type"`
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
