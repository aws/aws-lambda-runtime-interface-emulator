// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

type XrayTracingMode string

const (
	XRayTracingModeActive      XrayTracingMode = "Active"
	XRayTracingModePassThrough XrayTracingMode = "PassThrough"
)

type ArtefactType string

const (
	ArtefactTypeOCI ArtefactType = "oci"
	ArtefactTypeZIP ArtefactType = "zip"
)
