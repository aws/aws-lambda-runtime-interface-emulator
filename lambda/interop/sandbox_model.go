// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"time"

	"go.amzn.com/lambda/fatalerror"
)

// Init represents an init message
// In Rapid Shim, this is a START GirD message
// In Rapid Daemon, this is an INIT GirP message
type Init struct {
	InvokeID          string
	Handler           string
	AwsKey            string
	AwsSecret         string
	AwsSession        string
	CredentialsExpiry time.Time
	SuppressInit      bool
	InvokeTimeoutMs   int64  // timeout duration of whole invoke
	InitTimeoutMs     int64  // timeout duration for init only
	XRayDaemonAddress string // only in standalone
	FunctionName      string // only in standalone
	FunctionVersion   string // only in standalone
	// In standalone mode, these env vars come from test/init but from environment otherwise.
	CustomerEnvironmentVariables map[string]string
	SandboxType                  SandboxType
	// there is no dynamic config at the moment for the runtime domain
	OperatorDomainExtraConfig DynamicDomainConfig
	RuntimeInfo               RuntimeInfo
	Bootstrap                 Bootstrap
	EnvironmentVariables      EnvironmentVariables // contains env vars for agents and runtime procs
}

// InitStarted contains metadata about the initialized sandbox
// In Rapid Shim, this translates to a RUNNING GirD message to Slicer
// In Rapid Daemon, this is followed by a SANDBOX GirP message to MM
type InitStarted struct {
	WaitStartTimeNs   int64
	WaitEndTimeNs     int64
	PreLoadTimeNs     int64
	PostLoadTimeNs    int64
	ExtensionsEnabled bool
	Ack               chan struct{} // used by the sending goroutine to wait until ipc message has been sent
}

// InitSuccess indicates that runtime/extensions initialization completed successfully
// In Rapid Shim, this translates to a DONE GirD message to Slicer
// In Rapid Daemon, this is followed by a DONEDONE GirP message to MM
type InitSuccess struct {
	NumActiveExtensions int    // indicates number of active extensions
	ExtensionNames      string // file names of extensions in /opt/extensions
	RuntimeRelease      string
	LogsAPIMetrics      TelemetrySubscriptionMetrics // used if telemetry API enabled
	Ack                 chan struct{}                // used by the sending goroutine to wait until ipc message has been sent
}

// InitFailure indicates that runtime/extensions initialization failed due to process exit or /error calls
// In Rapid Shim, this translates to either a DONE or a DONEFAIL GirD message to Slicer (depending on extensions mode)
// However, even on failure, the next invoke is expected to work with a suppressed init - i.e. we init again as aprt of the invoke
type InitFailure struct {
	ResetReceived       bool // indicates if failure happened due to a reset received
	RequestReset        bool // Indicates whether reset should be requested on init failure
	ErrorType           fatalerror.ErrorType
	ErrorMessage        error
	NumActiveExtensions int
	RuntimeRelease      string // value of the User Agent HTTP header provided by runtime
	LogsAPIMetrics      TelemetrySubscriptionMetrics
	Ack                 chan struct{} // used by the sending goroutine to wait until ipc message has been sent
}

// ResponseMetrics groups metrics related to the response stream
type ResponseMetrics struct {
	RuntimeTimeThrottledMs       int64
	RuntimeProducedBytes         int64
	RuntimeOutboundThroughputBps int64
}

// InvokeMetrics groups metrics related to the invoke phase
type InvokeMetrics struct {
	InvokeRequestReadTimeNs int64
	InvokeRequestSizeBytes  int64
	RuntimeReadyTime        int64
}

// InvokeSuccess is the success response to invoke phase end
type InvokeSuccess struct {
	RuntimeRelease         string // value of the User Agent HTTP header provided by runtime
	NumActiveExtensions    int
	ExtensionNames         string
	InvokeCompletionTimeNs int64
	InvokeReceivedTime     int64
	LogsAPIMetrics         TelemetrySubscriptionMetrics
	ResponseMetrics        ResponseMetrics
	InvokeMetrics          InvokeMetrics
}

// InvokeFailure is the failure response to invoke phase end
type InvokeFailure struct {
	ResetReceived        bool // indicates if failure happened due to a reset received
	RequestReset         bool // indicates if reset must be requested after the failure
	ErrorType            fatalerror.ErrorType
	ErrorMessage         error
	RuntimeRelease       string // value of the User Agent HTTP header provided by runtime
	NumActiveExtensions  int
	InvokeReceivedTime   int64
	LogsAPIMetrics       TelemetrySubscriptionMetrics
	ResponseMetrics      ResponseMetrics
	InvokeMetrics        InvokeMetrics
	ExtensionNames       string
	DefaultErrorResponse *ErrorResponse // error resp constructed by platform during fn errors
}

// ResetSuccess is the success response to reset request
type ResetSuccess struct {
	ExtensionsResetMs int64
	ErrorType         fatalerror.ErrorType
	ResponseMetrics   ResponseMetrics
}

// ResetFailure is the failure response to reset request
type ResetFailure struct {
	ExtensionsResetMs int64
	ErrorType         fatalerror.ErrorType
	ResponseMetrics   ResponseMetrics
}

// ShutdownSuccess is the response to a shutdown request
type ShutdownSuccess struct {
	ErrorType fatalerror.ErrorType
}

// SandboxInfoFromInit captures data from init request that
// is required during invoke (e.g. for suppressed init)
type SandboxInfoFromInit struct {
	EnvironmentVariables EnvironmentVariables // contains agent env vars (creds, customer, platform)
	SandboxType          SandboxType          // indicating Pre-Warmed, On-Demand etc
	RuntimeBootstrap     Bootstrap            // contains the runtime bootstrap binary path, Cwd, Args, Env, Cmd
}

// RapidContext expose methods for functionality of the Rapid Core library
type RapidContext interface {
	HandleInit(i *Init, started chan<- InitStarted, success chan<- InitSuccess, failure chan<- InitFailure)
	HandleInvoke(i *Invoke, sbMetadata SandboxInfoFromInit) (InvokeSuccess, *InvokeFailure)
	HandleReset(reset *Reset, invokeReceivedTime int64, InvokeResponseMetrics *InvokeResponseMetrics) (ResetSuccess, *ResetFailure)
	HandleShutdown(shutdown *Shutdown) ShutdownSuccess
	HandleRestore(restore *Restore) error
	Clear()
}

// SandboxContext represents the sandbox lifecycle context
type SandboxContext interface {
	Init(i *Init, timeoutMs int64) (InitStarted, InitContext)
	Reset(reset *Reset) (ResetSuccess, *ResetFailure)
	Shutdown(shutdown *Shutdown) ShutdownSuccess
	Restore(restore *Restore) error

	// TODO: refactor this
	// invokeReceivedTime and InvokeResponseMetrics are needed to compute the runtimeDone metrics
	// in case of a Reset during an invoke (reset.reason=failure or reset.reason=timeout).
	// Ideally:
	// - the InvokeContext will have a Reset method to deal with Reset during an invoke and will hold invokeReceivedTime and InvokeResponseMetrics
	// - the SandboxContext will have its own Reset/Spindown method
	SetInvokeReceivedTime(invokeReceivedTime int64)
	SetInvokeResponseMetrics(metrics *InvokeResponseMetrics)
}

// InitContext represents the lifecycle of a sandbox initialization
type InitContext interface {
	Wait() (InitSuccess, *InitFailure)
	Reserve() InvokeContext
}

// InvokeContext represents the lifecycle of a sandbox reservation
type InvokeContext interface {
	SendRequest(i *Invoke)
	Wait() (InvokeSuccess, *InvokeFailure)
}

// Restored message is sent to Slicer to inform Runtime Restore Hook execution was successful
type Restored struct {
}
