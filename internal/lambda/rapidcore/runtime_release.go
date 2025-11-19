// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Logging string

const (
	AmznStdout    Logging = "amzn-stdout"
	AmznStdoutTLV Logging = "amzn-stdout-tlv"
)

// RuntimeRelease stores runtime identification data
type RuntimeRelease struct {
	Name    string
	Version string
	Logging Logging
}

const RuntimeReleasePath = "/var/runtime/runtime-release"

// GetRuntimeRelease reads Runtime identification data from config file and parses it into a struct
func GetRuntimeRelease(path string) (*RuntimeRelease, error) {
	pairs, err := ParsePropertiesFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}

	return &RuntimeRelease{pairs["NAME"], pairs["VERSION"], Logging(pairs["LOGGING"])}, nil
}

// ParsePropertiesFile reads key-value pairs from file in newline-separated list of environment-like
// shell-compatible variable assignments.
// Format: https://www.freedesktop.org/software/systemd/man/os-release.html
// Value quotes are trimmed. Latest write wins for duplicated keys.
func ParsePropertiesFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", path, err)
	}
	defer f.Close()

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
