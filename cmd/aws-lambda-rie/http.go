// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"
)

func startHTTPServer(ipport string, sandbox *rapidcore.SandboxBuilder, bs interop.Bootstrap) {
	srv := &http.Server{
		Addr: ipport,
	}

	// Pass a channel
	http.HandleFunc("/2015-03-31/functions/function/invocations", func(w http.ResponseWriter, r *http.Request) {
		InvokeHandler(w, r, sandbox.LambdaInvokeAPI(), bs)
	})

	// go routine (main thread waits)
	if err := srv.ListenAndServe(); err != nil {
		log.Panic(err)
	}

	log.Warnf("Listening on %s", ipport)
}
