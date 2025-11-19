// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"net"
)

// Test helpers
type TestSocketsRapid struct {
	CtrlFd int
	CnslFd int
}

type TestSocketsSlicer struct {
	CtrlSock net.Conn
	CnslSock net.Conn
	CtrlFd   int
	CnslFd   int
}
