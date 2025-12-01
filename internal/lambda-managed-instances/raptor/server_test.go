// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	"math"
	"math/rand/v2"
	"net/http"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testutils"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testutils/mocks"
)

func TestStartNewServer_UDS(t *testing.T) {
	var err error
	socketPath, err := testutils.CreateTempSocketPath(t)
	require.NoError(t, err)

	mockShutdownHandler := newMockShutdownHandler(t)
	mockShutdownHandler.On("Shutdown", mock.Anything).Return().Maybe()
	handler := mocks.NewNoOpHandler()

	server, err := StartServer(mockShutdownHandler, handler, &UnixAddress{
		Path: socketPath,
	})
	require.NoError(t, err)
	assert.Equal(t, socketPath, server.Addr.String())

	client := testutils.NewUnixSocketClient(socketPath)
	req, err := http.NewRequest("GET", "http://unix/", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.NoError(t, err)
}

func TestStartNewServer_TCP(t *testing.T) {
	var err error
	port := uint16(rand.UintN(math.MaxUint16-1024)) + 1024
	eaAPIAddrPort := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), port)

	mockShutdownHandler := newMockShutdownHandler(t)
	mockShutdownHandler.On("Shutdown", mock.Anything).Return().Maybe()
	handler := mocks.NewNoOpHandler()

	server, err := StartServer(mockShutdownHandler, handler, &TCPAddress{
		eaAPIAddrPort,
	})
	require.NoError(t, err)
	assert.Equal(t, eaAPIAddrPort, server.Addr.(*TCPAddress).AddrPort)

	_, err = http.Get("http://" + server.Addr.String())
	require.NoError(t, err)
}

func TestStartNewServer_UDS_ListenError(t *testing.T) {
	invalidSocketPath := filepath.Join("/nonexistent", "socket.sock")

	mockShutdownHandler := newMockShutdownHandler(t)
	handler := mocks.NewNoOpHandler()

	_, err := StartServer(mockShutdownHandler, handler, &UnixAddress{
		Path: invalidSocketPath,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), invalidSocketPath)
}

func TestStartNewServe_TCP_ListenError(t *testing.T) {
	eaAPIAddrPort := netip.MustParseAddrPort("1.1.1.1:49275")

	mockShutdownHandler := newMockShutdownHandler(t)
	handler := mocks.NewNoOpHandler()

	_, err := StartServer(mockShutdownHandler, handler, &TCPAddress{eaAPIAddrPort})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "1.1.1.1:49275")
}
