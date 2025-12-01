// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func CreateTempSocketPath(t *testing.T) (string, error) {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.sock", uuid.New().String()))

	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil {
			t.Logf("could not cleanup unix socket file %s: %s", socketPath, err)
		}
	})

	if len(socketPath) > 104 {
		return "", fmt.Errorf("socket path is too long: %s", socketPath)
	}

	return socketPath, nil
}

func NewUnixSocketClient(socketPath string) *http.Client {
	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}

	transport := &http.Transport{
		DialContext:           dialer,
		DisableCompression:    true,
		ResponseHeaderTimeout: 5 * time.Second,
	}

	return &http.Client{
		Transport: transport,
	}
}
