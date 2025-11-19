// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package appctx

import (
	"sync"
)

// A Key type is used as a key for storing values in the application context.
type Key int

type InitType int

const (
	// AppCtxInvokeErrorTraceDataKey is used for storing deferred invoke error cause header value.
	// Only used by xray. TODO refactor xray interface so it doesn't use appctx
	AppCtxInvokeErrorTraceDataKey Key = iota

	// AppCtxRuntimeReleaseKey is used for storing runtime release information (parsed from User_Agent Http header string).
	AppCtxRuntimeReleaseKey

	// AppCtxInteropServerKey is used to store a reference to the interop server.
	AppCtxInteropServerKey

	// AppCtxResponseSenderKey is used to store a reference to the response sender
	AppCtxResponseSenderKey

	// AppCtxFirstFatalErrorKey is used to store first unrecoverable error message encountered to propagate it to slicer with DONE(errortype) or DONEFAIL(errortype)
	AppCtxFirstFatalErrorKey

	// AppCtxInitType is used to store the init type (init caching or plain INIT)
	AppCtxInitType

	// AppCtxSandbox type is used to store the sandbox type (SandboxClassic or SandboxPreWarmed)
	AppCtxSandboxType
)

// Possible values for AppCtxInitType key
const (
	Init InitType = iota
	InitCaching
)

// ApplicationContext is an application scope context.
type ApplicationContext interface {
	Store(key Key, value interface{})
	Load(key Key) (value interface{}, ok bool)
	Delete(key Key)
	GetOrDefault(key Key, defaultValue interface{}) interface{}
	StoreIfNotExists(key Key, value interface{}) interface{}
}

type applicationContext struct {
	mux *sync.Mutex
	m   map[Key]interface{}
}

func (appCtx *applicationContext) Store(key Key, value interface{}) {
	appCtx.mux.Lock()
	defer appCtx.mux.Unlock()
	appCtx.m[key] = value
}

func (appCtx *applicationContext) StoreIfNotExists(key Key, value interface{}) interface{} {
	appCtx.mux.Lock()
	defer appCtx.mux.Unlock()
	existing, found := appCtx.m[key]
	if found {
		return existing
	}
	appCtx.m[key] = value
	return nil
}

func (appCtx *applicationContext) Load(key Key) (value interface{}, ok bool) {
	appCtx.mux.Lock()
	defer appCtx.mux.Unlock()
	value, ok = appCtx.m[key]
	return
}

func (appCtx *applicationContext) Delete(key Key) {
	appCtx.mux.Lock()
	defer appCtx.mux.Unlock()
	delete(appCtx.m, key)
}

func (appCtx *applicationContext) GetOrDefault(key Key, defaultValue interface{}) interface{} {
	if value, ok := appCtx.Load(key); ok {
		return value
	}
	return defaultValue
}

// NewApplicationContext returns a new instance of application context.
func NewApplicationContext() ApplicationContext {
	return &applicationContext{
		mux: &sync.Mutex{},
		m:   make(map[Key]interface{}),
	}
}
