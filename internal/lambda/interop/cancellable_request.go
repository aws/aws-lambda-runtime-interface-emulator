// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"net"
	"net/http"
)

type key int

const (
	HTTPConnKey key = iota
)

func GetConn(r *http.Request) net.Conn {
	return r.Context().Value(HTTPConnKey).(net.Conn)
}

type CancellableRequest struct {
	Request *http.Request
}

func (c *CancellableRequest) Cancel() error {
	return GetConn(c.Request).Close()
}
