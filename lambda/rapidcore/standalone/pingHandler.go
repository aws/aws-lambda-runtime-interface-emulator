// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"
)

func PingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}
