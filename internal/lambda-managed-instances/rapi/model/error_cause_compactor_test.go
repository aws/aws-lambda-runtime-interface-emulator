// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCauseCropMessageAndWorkingDir(t *testing.T) {
	largeString := strings.Repeat("a", 4*MaxErrorCauseSizeBytes)
	factorsAndExpectedLengths := map[float64]int{
		1.5: len(largeString),
		1.0: len(largeString),
		0.5: len(largeString),
		0.0: (MaxErrorCauseSizeBytes - paddingForFieldNames) / 2,
	}

	for factor, length := range factorsAndExpectedLengths {
		cause := ErrorCause{
			Message:    largeString,
			WorkingDir: largeString,
		}

		compactor := newErrorCauseCompactor(cause)
		compactor.crop(factor)

		failureMsg := fmt.Sprintf("factor: %f, length: expected=%d, actual=%d", factor, length, len(compactor.ec.Message))
		assert.Len(t, compactor.ec.Message, length, "Message: "+failureMsg)
		assert.Len(t, compactor.ec.WorkingDir, length, "WorkingDir: "+failureMsg)
	}
}

func TestErrorCauseCropStackTraces(t *testing.T) {
	noOfElements := 3 * MaxErrorCauseSizeBytes
	largeExceptions := make([]exception, noOfElements)
	for i := range largeExceptions {
		largeExceptions[i] = exception{Message: "a"}
	}

	largePaths := make([]string, noOfElements)
	for i := range largePaths {
		largePaths[i] = "a"
	}

	factorsAndExpectedLengths := map[float64]int{
		1.5: noOfElements,
		1.0: noOfElements,
		0.5: int(noOfElements / 2),
		0.0: 0,
	}

	for factor, length := range factorsAndExpectedLengths {
		cause := ErrorCause{
			Exceptions: largeExceptions,
			Paths:      largePaths,
		}

		compactor := newErrorCauseCompactor(cause)
		compactor.crop(factor)

		failureMsg := fmt.Sprintf("factor: %f, length: expected=%d, actual=%d", factor, length, len(compactor.ec.WorkingDir))
		assert.Len(t, compactor.ec.Exceptions, length, "Exceptions: "+failureMsg)
		assert.Len(t, compactor.ec.Paths, length, "Paths: "+failureMsg)
	}
}

func TestCropString(t *testing.T) {
	maxLen := 5
	stringsAndExpectedCrops := map[string]string{
		"abcde":  "abcde",
		"abcdef": "ab...",
		"":       "",
	}

	for str, expectedStr := range stringsAndExpectedCrops {
		assert.Equal(t, expectedStr, cropString(str, maxLen))
	}
}
