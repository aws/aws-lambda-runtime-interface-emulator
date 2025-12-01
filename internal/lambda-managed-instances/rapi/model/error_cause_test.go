// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCauseValidationWhenCauseIsValid(t *testing.T) {
	validCauses := [][]byte{
		[]byte(`{"paths":[],"working_directory":"/foo/bar/baz","exceptions":[]}`),
		[]byte(`{"paths":["foo", "bar"]}`),
		[]byte(`{"working_directory":"/foo/bar/baz"}`),
		[]byte(`{"exceptions":[{"message": "foo"}, {"message": "bar"}]}`),
		[]byte(`{"exceptions":[{}]}`),
		[]byte(`{"exceptions":[{}], "arbitrary":"field"}`),
		[]byte(`{"message":"foo error"}`),
	}

	for _, c := range validCauses {
		_, err := ValidatedErrorCauseJSON(c)
		assert.Nil(t, err, "validation failed for valid cause")
	}
}

func TestWorkingDirCropping(t *testing.T) {
}

func TestErrorCauseMarshallingWhenCauseIsValid(t *testing.T) {
	causesAndExpectations := map[string]string{
		`{"paths":[],"working_directory":"/","exceptions":[]}`: `{"paths":[],"working_directory":"/","exceptions":[]}`,
		`{"paths":["f"]}`:                          `{"paths":["f"],"working_directory":"","exceptions":null}`,
		`{"working_directory":"/foo"}`:             `{"paths":null,"working_directory":"/foo","exceptions":null}`,
		`{"exceptions":[{}], "arbitrary":"field"}`: `{"paths":null,"working_directory":"","exceptions":[{}]}`,
		`{"message":"foo"}`:                        `{"paths":null,"working_directory":"","exceptions":null,"message":"foo"}`,
	}

	for causeJSON, expectedJSON := range causesAndExpectations {
		validCauseJSON, err := ValidatedErrorCauseJSON([]byte(causeJSON))
		assert.Nil(t, err, "validation failed for valid cause")
		assert.JSONEq(t, string(expectedJSON), string(validCauseJSON))
	}
}

func TestErrorCauseValidationWhenCauseIsInvalid(t *testing.T) {
	invalidCauses := [][]byte{
		[]byte(`{"paths":[],"working_directory":"","exceptions":[]}`),
		[]byte(`{"paths":"","working_directory":"","exceptions":[]}`),
		[]byte(`{"paths":"","exceptions":[]}`),
		[]byte(`{foo: invalid}`),
		[]byte(`{}`),
		[]byte(`{"arbitrary":"field"}`),
	}

	for _, c := range invalidCauses {
		causeJSON, err := ValidatedErrorCauseJSON(c)
		assert.Error(t, err, "validation didn't return an error")
		assert.Nil(t, causeJSON)
	}
}

func TestErrorCauseCroppedJSONForEmptyCause(t *testing.T) {
	emptyCauseJSON := `{"exceptions":null, "paths":null, "working_directory":""}`
	cause := ErrorCause{}

	causeJSON := cause.croppedJSON()

	assert.JSONEq(t, emptyCauseJSON, string(causeJSON))
}

func TestErrorCauseCroppedJSONForLargeCause(t *testing.T) {
	noOfElements := MaxErrorCauseSizeBytes
	largeExceptions := make([]exception, noOfElements)
	for i := range largeExceptions {
		largeExceptions[i] = exception{Message: "a"}
	}

	largePaths := make([]string, noOfElements)
	for i := range largePaths {
		largePaths[i] = "a"
	}

	largeCause := ErrorCause{
		Message:    strings.Repeat("a", noOfElements),
		WorkingDir: strings.Repeat("a", noOfElements),
		Exceptions: largeExceptions,
		Paths:      largePaths,
	}
	expectedStringFieldsLen := (MaxErrorCauseSizeBytes - paddingForFieldNames) / 2

	causeJSON := largeCause.croppedJSON()
	assert.True(t, len(causeJSON) <= MaxErrorCauseSizeBytes, fmt.Sprintf("cropped JSON too long: len=%d", len(causeJSON)))

	parsedCause, err := newErrorCause(causeJSON)
	assert.NoError(t, err, "failed to parse constructed JSON")
	assert.Len(t, parsedCause.Message, expectedStringFieldsLen, "Message length incorrect")
	assert.Len(t, parsedCause.WorkingDir, expectedStringFieldsLen, "WorkingDir length incorrect")
	assert.Len(t, parsedCause.Exceptions, 0, "Exceptions length incorrect")
	assert.Len(t, parsedCause.Paths, 0, "Paths length incorrect")
}

func TestErrorCauseCroppedJSONForLargeCauseWithOnlyExceptionsAndPaths(t *testing.T) {
	elementsAndExpectedLengthFactors := map[int]float64{
		100:                        0.8,
		5000:                       0.6,
		8000:                       0.4,
		10000:                      0.2,
		MaxErrorCauseSizeBytes / 4: 0.0,
	}

	for noOfElements, factor := range elementsAndExpectedLengthFactors {
		largeExceptions := make([]exception, noOfElements)
		for i := range largeExceptions {
			largeExceptions[i] = exception{Message: "a"}
		}

		largePaths := make([]string, noOfElements)
		for i := range largePaths {
			largePaths[i] = "a"
		}

		largeCause := ErrorCause{
			Exceptions: largeExceptions,
			Paths:      largePaths,
		}

		causeJSON := largeCause.croppedJSON()
		assert.True(t, len(causeJSON) <= MaxErrorCauseSizeBytes, fmt.Sprintf("cropped JSON too long: len=%d", len(causeJSON)))

		parsedCause, err := newErrorCause(causeJSON)
		assert.NoError(t, err, "failed to parse constructed JSON")
		assert.Len(t, parsedCause.Message, 0, "Message length incorrect")
		assert.Len(t, parsedCause.WorkingDir, 0, "WorkingDir length incorrect")
		assert.Len(t, parsedCause.Exceptions, int(float64(noOfElements)*factor), "Exceptions length incorrect")
		assert.Len(t, parsedCause.Paths, int(float64(noOfElements)*factor), "Paths length incorrect")
	}
}
