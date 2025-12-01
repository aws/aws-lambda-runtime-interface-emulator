// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"io"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

type ExecutionEnvironmentAction interface {
	Execute(t *testing.T, client *Client) (*http.Response, error)
	ValidateStatus(t *testing.T, resp *http.Response)
	String() string
}

type RuntimeEnv struct {
	Workers []RuntimeExecutionEnvironment

	ForcedError      error
	T                *testing.T
	ExitProcessOnce  func()
	Stdout, Stderr   io.Writer
	InvokeResponseWG *sync.WaitGroup
	Done             sync.WaitGroup
}

type RuntimeExecutionEnvironment struct {
	Actions    []ExecutionEnvironmentAction
	InvokeID   interop.InvokeID
	RuntimeEnv *RuntimeEnv
}

func (r *RuntimeEnv) Exec(request *model.ExecRequest) (<-chan struct{}, error) {
	if r.ForcedError != nil {
		return nil, r.ForcedError
	}
	doneCh := make(chan struct{})
	r.ExitProcessOnce = sync.OnceFunc(func() {
		close(doneCh)
	})
	r.Stdout = request.StdoutWriter
	r.Stderr = request.StderrWriter
	runtimeAPIAddr := (*request.Env)["AWS_LAMBDA_RUNTIME_API"]
	client := NewClient(netip.MustParseAddrPort(runtimeAPIAddr))
	for _, thread := range r.Workers {
		r.Done.Add(1)
		go func() {
			thread.RuntimeEnv = r
			thread.executeEnvActions(client, r.T)
			r.Done.Done()
		}()
	}

	return doneCh, nil
}

func (r *RuntimeEnv) Terminate() error {
	r.ExitProcessOnce()
	return nil
}

func (r *RuntimeEnv) Kill() error {
	r.ExitProcessOnce()
	return nil
}

type ExtensionsEnv = map[string]*ExtensionsExecutionEnvironment

type ExtensionsExecutionEnvironment struct {
	Actions []ExecutionEnvironmentAction

	ForcedError error

	ExtensionIdentifier uuid.UUID
	T                   *testing.T
	ExitProcessOnce     func()
	Stdout, Stderr      io.Writer
	Done                sync.WaitGroup
}

func (e *ExtensionsExecutionEnvironment) Exec(request *model.ExecRequest) (<-chan struct{}, error) {
	if e.ForcedError != nil {
		return nil, e.ForcedError
	}

	runtimeAPIAddr := (*request.Env)["AWS_LAMBDA_RUNTIME_API"]
	doneCh := make(chan struct{})
	e.ExitProcessOnce = sync.OnceFunc(func() {
		close(doneCh)
	})
	e.Stdout = request.StdoutWriter
	e.Stderr = request.StderrWriter
	e.Done.Add(1)
	go func() {
		extensionClient := NewExtensionsClient(netip.MustParseAddrPort(runtimeAPIAddr))
		e.executeEnvActions(extensionClient, e.T)
		e.Done.Done()
	}()

	return doneCh, nil
}

func (e *ExtensionsExecutionEnvironment) Terminate() error {
	e.ExitProcessOnce()
	return nil
}

func (e *ExtensionsExecutionEnvironment) Kill() error {
	e.ExitProcessOnce()
	return nil
}

func (r *RuntimeExecutionEnvironment) executeEnvActions(client *Client, t *testing.T) {
	for _, action := range r.Actions {
		switch a := action.(type) {
		case NextAction:
			resp, err := a.Execute(t, client)
			require.NoError(t, err, "Action %s failed: %v", action.String(), err)

			if resp != nil {
				r.InvokeID = resp.Header.Get(invoke.RuntimeRequestIdHeader)
				a.ValidateStatus(t, resp)
			}
		case StdoutAction:
			a.stdout = r.RuntimeEnv.Stdout
			executeAndValidateAction(a, client, t)
		case StderrAction:
			a.stderr = r.RuntimeEnv.Stderr
			executeAndValidateAction(a, client, t)
		case InvocationResponseAction:
			if a.InvokeID == "" {
				a.InvokeID = r.InvokeID
			}
			executeAndValidateAction(a, client, t)
		case InvocationStreamingResponseAction:
			if a.InvokeID == "" {
				a.InvokeID = r.InvokeID
			}
			executeAndValidateAction(a, client, t)
		case InvocationResponseErrorAction:
			if a.InvokeID == "" {
				a.InvokeID = r.InvokeID
			}
			executeAndValidateAction(a, client, t)
		case ExitAction:
			a.exitProcessOnce = r.RuntimeEnv.ExitProcessOnce
			executeAndValidateAction(a, client, t)
		case WaitInvokeResponseAction:
			a.wg = r.RuntimeEnv.InvokeResponseWG
			executeAndValidateAction(&a, client, t)
		default:
			executeAndValidateAction(a, client, t)
		}
	}
}

func (e *ExtensionsExecutionEnvironment) executeEnvActions(client *Client, t *testing.T) {
	for _, action := range e.Actions {
		switch a := action.(type) {
		case StdoutAction:
			a.stdout = e.Stdout
			executeAndValidateAction(a, client, t)
		case StderrAction:
			a.stderr = e.Stderr
			executeAndValidateAction(a, client, t)
		case ExtensionsRegisterAction:
			resp, err := a.Execute(t, client)
			require.NoError(t, err)

			agentIdentifier := resp.Header.Get("Lambda-Extension-Identifier")
			if id, err := uuid.Parse(agentIdentifier); err == nil {
				e.ExtensionIdentifier = id
			}
			require.NoError(t, err)

			a.ValidateStatus(t, resp)
		case ExtensionsNextAction:
			if a.AgentIdentifier == "" {
				a.AgentIdentifier = e.ExtensionIdentifier.String()
			}
			executeAndValidateAction(a, client, t)
		case ExtensionsNextParallelAction:
			if a.AgentIdentifier == "" {
				a.AgentIdentifier = e.ExtensionIdentifier.String()
			}
			a.Environment = e
			executeAndValidateAction(a, client, t)
		case ExtensionsInitErrorAction:
			if a.AgentIdentifier == "" {
				a.AgentIdentifier = e.ExtensionIdentifier.String()
			}
			executeAndValidateAction(a, client, t)
		case ExtensionsExitErrorAction:
			if a.AgentIdentifier == "" {
				a.AgentIdentifier = e.ExtensionIdentifier.String()
			}
			executeAndValidateAction(a, client, t)
		case ExtensionsTelemetryAPIHTTPSubscriberAction:
			if a.Subscription.AgentIdentifier == "" {
				a.Subscription.AgentIdentifier = e.ExtensionIdentifier.String()
			}
			executeAndValidateAction(a, client, t)
		case ExtensionsTelemetryAPITCPSubscriberAction:
			if a.Subscription.AgentIdentifier == "" {
				a.Subscription.AgentIdentifier = e.ExtensionIdentifier.String()
			}
			executeAndValidateAction(a, client, t)
		case ExtensionTelemetrySubscribeAction:
			if a.AgentIdentifier == "" {
				a.AgentIdentifier = e.ExtensionIdentifier.String()
			}
			executeAndValidateAction(a, client, t)
		case ExitAction:
			a.exitProcessOnce = e.ExitProcessOnce
			executeAndValidateAction(a, client, t)
		default:
			executeAndValidateAction(a, client, t)
		}
	}
}

func executeAndValidateAction(action ExecutionEnvironmentAction, client *Client, t *testing.T) {
	resp, err := action.Execute(t, client)
	require.NoError(t, err, "Action %s failed: %v", action.String(), err)
	action.ValidateStatus(t, resp)
}

func MakeMockFileUtil(extensions ExtensionsEnv) *utils.MockFileUtil {
	return SetupMockFileUtil(extensions, true, true)
}

func SetupMockFileUtil(extensions ExtensionsEnv, hasBootstrap bool, hasVarTask bool) *utils.MockFileUtil {
	mockFileUtil := &utils.MockFileUtil{}

	var extensionEntries []os.DirEntry
	for extensionId := range extensions {
		extensionEntries = append(extensionEntries, utils.NewMockDirEntry(extensionId, false))
	}

	mockFileUtil.On("ReadDirectory", mock.MatchedBy(func(path string) bool {
		return strings.Contains(path, "/opt/extensions")
	})).Return(extensionEntries, nil)

	mockFileUtil.On("ReadDirectory", mock.Anything).Return(nil, nil)

	if hasBootstrap {

		bootstrapFileInfo := &utils.MockFileInfo{}
		bootstrapFileInfo.On("IsDir").Return(false)

		mockFileUtil.On("Stat", "/var/runtime/bootstrap").Return(bootstrapFileInfo, nil)
	} else {

		mockFileUtil.On("Stat", mock.MatchedBy(func(path string) bool {
			return path == "/var/runtime/bootstrap" || path == "/var/task/bootstrap" || path == "/opt/bootstrap"
		})).Return(nil, os.ErrNotExist)
	}

	if hasVarTask {
		mockFileUtil.On("Stat", "/var/task").Return(nil, nil)
	} else {
		mockFileUtil.On("Stat", "/var/task").Return(nil, os.ErrNotExist)
	}

	mockFileUtil.On("Stat", mock.Anything).Return(nil, nil)
	mockFileUtil.On("IsNotExist", mock.Anything).Return(os.IsNotExist)
	return mockFileUtil
}
