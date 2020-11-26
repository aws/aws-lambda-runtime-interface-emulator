// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"errors"
	"strings"
)

func SplitEnvironmentVariable(envKeyVal string) (string, string, error) {
	splitKeyVal := strings.SplitN(envKeyVal, "=", 2) // values can contain '='
	if len(splitKeyVal) < 2 {
		return "", "", errors.New("could not split env var by '=' delimiter")
	}
	return splitKeyVal[0], splitKeyVal[1], nil
}
