// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"net/http"

	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"
)

func standaloneAccessLogDecorator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("standalone: -> %s %s %v", r.Method, r.URL, r.Header)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		status := 200
		if ww.Status() != 0 {
			status = ww.Status()
		}

		if status != 0 && status/100 != 2 {
			log.Errorf("standalone: <- %s %d %v", r.URL, status, w.Header())
		} else {
			log.Debugf("standalone: <- %s %d %v", r.URL, status, w.Header())
		}
	})
}
