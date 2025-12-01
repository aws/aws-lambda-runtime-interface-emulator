// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
)

func SplitEnvironmentVariable(envKeyVal string) (string, string, error) {
	splitKeyVal := strings.SplitN(envKeyVal, "=", 2)
	if len(splitKeyVal) < 2 {
		return "", "", errors.New("could not split env var by '=' delimiter")
	}
	return splitKeyVal[0], splitKeyVal[1], nil
}

func KVPairStringsToMap(envKVPairs model.KVSlice) model.KVMap {
	bootstrapEnvMap := make(model.KVMap, len(envKVPairs))
	for _, es := range envKVPairs {
		key, val, err := SplitEnvironmentVariable(es)
		if err != nil {

			slog.Warn("invalid environment variable format", "err", err)
			continue
		}
		bootstrapEnvMap[key] = val
	}
	return bootstrapEnvMap
}

func MapToKVPairStrings(m model.KVMap) model.KVSlice {
	var env model.KVSlice
	for k, v := range m {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
