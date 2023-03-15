// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"fmt"
	"go.amzn.com/lambda/core"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi"
	"go.amzn.com/lambda/testdata"

	"github.com/stretchr/testify/assert"
)

func BenchmarkChannelsSelect10(b *testing.B) {
	c1 := make(chan int)
	c2 := make(chan int)
	c3 := make(chan int)
	c4 := make(chan int)
	c5 := make(chan int)
	c6 := make(chan int)
	c7 := make(chan int)
	c8 := make(chan int)
	c9 := make(chan int)
	c10 := make(chan int)

	for n := 0; n < b.N; n++ {
		select {
		case <-c1:
			break
		case <-c2:
			break
		case <-c3:
			break
		case <-c4:
			break
		case <-c5:
			break
		case <-c6:
			break
		case <-c7:
			break
		case <-c8:
			break
		case <-c9:
			break
		case <-c10:
			break
		default:
			break
		}
	}
}

func BenchmarkChannelsSelect2(b *testing.B) {
	c1 := make(chan int)
	c2 := make(chan int)

	for n := 0; n < b.N; n++ {
		select {
		case <-c1:
			break
		case <-c2:
			break
		default:
			break
		}
	}
}

func TestGetExtensionNamesWithNoExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil, nil)

	c := &rapidContext{
		registrationService: rs,
	}

	assert.Equal(t, "", c.GetExtensionNames())
}

func TestGetExtensionNamesWithMultipleExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil, nil)
	_, _ = rs.CreateExternalAgent("Example1")
	_, _ = rs.CreateInternalAgent("Example2")
	_, _ = rs.CreateExternalAgent("Example3")
	_, _ = rs.CreateInternalAgent("Example4")

	c := &rapidContext{
		registrationService: rs,
	}

	r := regexp.MustCompile(`^(Example\d;){3}(Example\d)$`)
	assert.True(t, r.MatchString(c.GetExtensionNames()))
}

func TestGetExtensionNamesWithTooManyExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil, nil)
	for i := 10; i < 60; i++ {
		_, _ = rs.CreateExternalAgent("E" + strconv.Itoa(i))
	}

	c := &rapidContext{
		registrationService: rs,
	}

	output := c.GetExtensionNames()

	r := regexp.MustCompile(`^(E\d\d;){30}(E\d\d)$`)
	assert.LessOrEqual(t, len(output), maxExtensionNamesLength)
	assert.True(t, r.MatchString(output))
}

func TestGetExtensionNamesWithTooLongExtensionName(t *testing.T) {
	rs := core.NewRegistrationService(nil, nil)
	for i := 10; i < 60; i++ {
		_, _ = rs.CreateExternalAgent(strings.Repeat("E", 130))
	}

	c := &rapidContext{
		registrationService: rs,
	}

	assert.Equal(t, "", c.GetExtensionNames())
}

// This test confirms our assumption that http client can establish a tcp connection
// to a listening server.
func TestListen(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.ConfigureForInvoke(context.Background(), &interop.Invoke{ID: "ID", DeadlineNs: "1", Payload: strings.NewReader("MyTest")})

	ctx := context.Background()
	telemetryAPIEnabled := true
	server := rapi.NewServer("127.0.0.1", 0, flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService, telemetryAPIEnabled, flowTest.TelemetrySubscription, flowTest.TelemetrySubscription, flowTest.CredentialsService, flowTest.EventsAPI)
	err := server.Listen()
	assert.NoError(t, err)

	defer server.Close()

	go func() {
		time.Sleep(time.Second)
		fmt.Println("Serving...")
		server.Serve(ctx)
	}()

	done := make(chan struct{})

	go func() {
		fmt.Println("Connecting...")
		resp, err1 := http.Get(fmt.Sprintf("http://%s:%d/2018-06-01/runtime/invocation/next", server.Host(), server.Port()))
		assert.Nil(t, err1)

		body, err2 := io.ReadAll(resp.Body)
		assert.Nil(t, err2)

		assert.Equal(t, "MyTest", string(body))

		done <- struct{}{}
	}()

	<-done
}

func TestInferSandboxInitTypeOnDemand(t *testing.T) {
	initCachingEnabled := false
	sandboxType := interop.SandboxClassic
	initSource := interop.InferTelemetryInitSource(initCachingEnabled, sandboxType)
	assert.Equal(t, "on-demand", initSource)
}

func TestInferSandboxInitTypeProvisionedConcurrency(t *testing.T) {
	initCachingEnabled := false
	sandboxType := interop.SandboxPreWarmed
	initSource := interop.InferTelemetryInitSource(initCachingEnabled, sandboxType)
	assert.Equal(t, "provisioned-concurrency", initSource)
}

func TestInferSandboxInitTypeInitCaching(t *testing.T) {
	initCachingEnabled := true
	sandboxType := interop.SandboxClassic
	initSource := interop.InferTelemetryInitSource(initCachingEnabled, sandboxType)
	assert.Equal(t, "snap-start", initSource)
}

func TestInferSandboxInitTypeInitCachingWithPC(t *testing.T) {
	initCachingEnabled := true
	sandboxType := interop.SandboxPreWarmed
	initSource := interop.InferTelemetryInitSource(initCachingEnabled, sandboxType)
	assert.Equal(t, "snap-start", initSource)
}
