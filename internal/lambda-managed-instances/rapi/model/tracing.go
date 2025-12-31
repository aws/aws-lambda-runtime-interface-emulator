// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

type TracingType string

const (
	XRayTracingType TracingType = "X-Amzn-Trace-Id"
)

const (
	XRaySampled    = "1"
	XRayNonSampled = "0"
)

type Tracing struct {
	Type TracingType `json:"type"`
	XRayTracing
}

type XRayTracing struct {
	Value string `json:"value"`
}

func NewXRayTracing(value string) *Tracing {
	if len(value) == 0 {
		return nil
	}

	return &Tracing{
		XRayTracingType,
		XRayTracing{value},
	}
}
