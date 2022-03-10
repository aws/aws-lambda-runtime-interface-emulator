// Simple testing utility to emulate the internal LocalStack Endpoint
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

const apiPort = 9563
const listenPort = 48490

var invokeUrl = fmt.Sprintf("http://localhost:%d/invoke", apiPort)

func main() {
	// mock for Localstack component

	uid := "12345"

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Post("/invocations/{invoke_id}/response", invokeResponseHandler)
	router.Post("/invocations/{invoke_id}/error", invokeErrorHandler)
	router.Post("/invocations/{invoke_id}/logs", invokeLogsHandler)
	router.Post("/status/{runtime_id}/{status}", statusHandler)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		invokeRequest, _ := json.Marshal(InvokeRequest{InvokeId: uid, Payload: "{\"counter\":0}"})
		_, err := http.Post(invokeUrl, "application/json", bytes.NewReader(invokeRequest))
		if err != nil {
			log.Fatal(err)
		}

		w.WriteHeader(200)
		_, err = w.Write([]byte("hi"))
		if err != nil {
			log.Error(err)
		}
	})

	router.Get("/fail", func(w http.ResponseWriter, r *http.Request) {
		invokeRequest, _ := json.Marshal(InvokeRequest{InvokeId: uid, Payload: "{\"counter\":0, \"fail\": \"yes\"}"})
		_, err := http.Post(invokeUrl, "application/json", bytes.NewReader(invokeRequest))
		if err != nil {
			log.Fatal(err)
		}

		w.WriteHeader(200)
		_, err = w.Write([]byte("hi"))
		if err != nil {
			log.Error(err)
		}
	})

	err := http.ListenAndServe(fmt.Sprintf(":%d", listenPort), router)
	if err != nil {
		log.Fatal(err)
	}
}

func invokeLogsHandler(w http.ResponseWriter, r *http.Request) {
	invokeId := chi.URLParam(r, "invoke_id")
	log.Println(invokeId)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
	}
	log.Println("log result: " + string(bodyBytes))
}

type InvokeRequest struct {
	InvokeId string `json:"invoke-id"`
	Payload  string `json:"payload"`
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	runtime_id := chi.URLParam(r, "runtime_id")
	status := chi.URLParam(r, "status")
	log.Println(runtime_id + " + " + status)
	if status == "ready" {
		go func() {
			invokeRequest, _ := json.Marshal(InvokeRequest{InvokeId: "12345", Payload: "{\"counter\":0}"})
			_, err := http.Post(invokeUrl, "application/json", bytes.NewReader(invokeRequest))
			if err != nil {
				log.Fatal(err)
			}
		}()
	}
}

func invokeResponseHandler(w http.ResponseWriter, r *http.Request) {
	invokeId := chi.URLParam(r, "invoke_id")
	log.Println(invokeId)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
	}
	log.Println("result: " + string(bodyBytes))
}

func invokeErrorHandler(w http.ResponseWriter, r *http.Request) {
	invokeId := chi.URLParam(r, "invoke_id")
	log.Println(invokeId)
}
