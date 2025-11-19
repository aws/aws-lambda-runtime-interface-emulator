// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"math"
)

const paddingForFieldNames = 4096

type errorCauseCompactor struct {
	ec ErrorCause
}

func newErrorCauseCompactor(errorCause ErrorCause) *errorCauseCompactor {
	ec := errorCause
	return &errorCauseCompactor{ec}
}

// cropStackTraces crops Exceptions & Paths of an error cause
// by a factor in [0,1]. e.g. 0 removes all elements, 1 removes
// nothing, 0.5 removes half the array elements
func (c *errorCauseCompactor) cropStackTraces(factor float64) {
	if factor > 0 {
		factor = math.Min(factor, 1) // number in (0,1]
		exceptionsLen := float64(len(c.ec.Exceptions)) * factor
		pathLen := float64(len(c.ec.Paths)) * factor

		c.ec.Exceptions = c.ec.Exceptions[:int(exceptionsLen)]
		c.ec.Paths = c.ec.Paths[:int(pathLen)]

		return
	}

	c.ec.Exceptions = nil
	c.ec.Paths = nil
}

// cropMessage crops Message of an error cause to be half the
// max size when the factor is 0 (worst-case)
func (c *errorCauseCompactor) cropMessage(factor float64) {
	if factor > 0 {
		return
	}

	length := ((MaxErrorCauseSizeBytes - paddingForFieldNames) / 2)
	c.ec.Message = cropString(c.ec.Message, length)
}

// cropWorkingDir crops WorkingDir of an error cause to be half
// the max size when the factor is 0 (worst-case)
func (c *errorCauseCompactor) cropWorkingDir(factor float64) {
	if factor > 0 {
		return
	}

	length := ((MaxErrorCauseSizeBytes - paddingForFieldNames) / 2)
	c.ec.WorkingDir = cropString(c.ec.WorkingDir, length)
}

func (c *errorCauseCompactor) crop(factor float64) {
	c.cropStackTraces(factor)
	c.cropMessage(factor)
	c.cropWorkingDir(factor)
}

func (c *errorCauseCompactor) cause() *ErrorCause {
	return &c.ec
}

func cropString(str string, length int) string {
	if len(str) <= length {
		return str
	}

	truncationIndicator := `...`
	length = length - len(truncationIndicator)
	return str[:length] + truncationIndicator
}
