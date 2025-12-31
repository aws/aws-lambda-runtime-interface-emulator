// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package appctx

import (
	"sync"
)

type Key int

const (
	AppCtxInvokeErrorTraceDataKey Key = iota

	AppCtxRuntimeReleaseKey

	AppCtxInteropServerKey

	AppCtxResponseSenderKey

	AppCtxFirstFatalErrorKey
)

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
	return value, ok
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

func NewApplicationContext() ApplicationContext {
	return &applicationContext{
		mux: &sync.Mutex{},
		m:   make(map[Key]interface{}),
	}
}
