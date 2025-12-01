// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"testing"
	"time"
)

func WaitForErrorWithTimeout(channel <-chan error, timeout time.Duration) error {
	select {
	case err := <-channel:
		return err
	case <-time.After(timeout):
		return nil
	}
}

func Eventually(t *testing.T, testFunc func() (bool, error), pollingIntervalMultiple time.Duration, retries int) bool {
	for try := 0; try < retries; try++ {
		success, err := testFunc()
		if success {
			return true
		}
		if err != nil {
			t.Logf("try %d: %v", try, err)
		}
		time.Sleep(time.Duration(try) * pollingIntervalMultiple)
	}
	return false
}
