package main

import (
	"strings"
	"sync"
)

type LogResponse struct {
	Logs string `json:"logs"`
}

type LogCollector struct {
	mutex       *sync.Mutex
	RuntimeLogs []string
}

func (lc *LogCollector) Write(p []byte) (n int, err error) {
	lc.Put(string(p))
	return len(p), nil
}

func NewLogCollector() *LogCollector {
	return &LogCollector{
		RuntimeLogs: []string{},
		mutex:       &sync.Mutex{},
	}
}
func (lc *LogCollector) Put(line string) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	lc.RuntimeLogs = append(lc.RuntimeLogs, line)
}

func (lc *LogCollector) reset() {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	lc.RuntimeLogs = []string{}
}

func (lc *LogCollector) getLogs() LogResponse {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	response := LogResponse{
		Logs: strings.Join(lc.RuntimeLogs, ""),
	}
	lc.RuntimeLogs = []string{}
	return response
}
