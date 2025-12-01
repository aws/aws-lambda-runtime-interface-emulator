// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Logging string

const (
	AmznStdout    Logging = "amzn-stdout"
	AmznStdoutTLV Logging = "amzn-stdout-tlv"
)

type RuntimeRelease struct {
	Name    string
	Version string
	Logging Logging
}

func (rr *RuntimeRelease) GetUAProduct() (string, error) {
	if rr.Name == "" {
		return "", errors.New("runtime release name is empty")
	}
	if rr.Version == "" {
		return rr.Name, nil
	}

	return fmt.Sprintf("%s/%s", rr.Name, rr.Version), nil
}

const RuntimeReleasePath = "/var/runtime/runtime-release"

const runtimeReleaseFileSizeLimitBytes = 1024

func GetRuntimeRelease(path string) (*RuntimeRelease, error) {

	pairs, err := ParsePropertiesFile(path, runtimeReleaseFileSizeLimitBytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}

	return &RuntimeRelease{pairs["NAME"], pairs["VERSION"], Logging(pairs["LOGGING"])}, nil
}

func GetRuntimeLoggingType(rr *RuntimeRelease) Logging {
	if rr == nil {
		return AmznStdout
	}
	return rr.Logging
}

func ParsePropertiesFile(path string, limitBytes int64) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("could not close file", "path", path, "err", err)
		}
	}()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat file: %s", path)
	}
	if stat.Size() > limitBytes {
		return nil, fmt.Errorf("file %s size %d > %d limit", path, stat.Size(), limitBytes)
	}

	pairs := make(map[string]string)

	s := bufio.NewScanner(f)
	for s.Scan() {
		if s.Text() == "" || strings.HasPrefix(s.Text(), "#") {
			continue
		}
		k, v, found := strings.Cut(s.Text(), "=")
		if !found {
			return nil, fmt.Errorf("could not parse key-value pair from a line: %s", s.Text())
		}
		pairs[k] = strings.Trim(v, "'\"")
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("failed to read properties file: %w", err)
	}

	return pairs, nil
}
