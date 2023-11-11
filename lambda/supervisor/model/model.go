// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

// Start, Stop and Configure methods are not used in Core anymore.
// Client interface splitted into Launcher and Executer parts for backward compatibility of dependent packages.
type ContainerSupervisor interface {
	Start(context.Context, *StartRequest) error
	Configure(context.Context, *ConfigureRequest) error
	Stop(context.Context, *StopRequest) (*StopResponse, error)
	Freeze(context.Context, *FreezeRequest) (*FreezeResponse, error)
	Thaw(context.Context, *ThawRequest) error
	Exit(context.Context)
}

type ProcessSupervisor interface {
	Exec(context.Context, *ExecRequest) error
	Terminate(context.Context, *TerminateRequest) error
	Kill(context.Context, *KillRequest) error
	Events(context.Context, *EventsRequest) (<-chan Event, error)
}

type SupervisorClient interface {
	ContainerSupervisor
	ProcessSupervisor
	Ping(ctx context.Context) error
}

type StartRequest struct {
	Domain string `json:"domain"`
}

type Mount struct {
	DriveMount DriveMount
	BindMount  BindMount
	MountType  MountType
}

type MountType int

const (
	_ MountType = iota
	MountTypeDrive
	MountTypeBind
)

type CgroupProfileName string

const (
	Throttled   CgroupProfileName = "throttled"
	Unthrottled CgroupProfileName = "unthrottled"
)

func (m *Mount) MarshalJSON() ([]byte, error) {
	switch m.MountType {
	case MountTypeDrive:
		return m.DriveMount.MarshalJSON()
	case MountTypeBind:
		return m.BindMount.MarshalJSON()
	default:
		return nil, fmt.Errorf("invalid mount type: %v", m.MountType)
	}
}

// Mount in lockhard::mnt is a Rust enum, an algebraic type, where each case has different set of fields.
// This models only the Mount::Drive case, the only one we need for now.
type DriveMount struct {
	Source      string   `json:"source,omitempty"`
	Destination string   `json:"destination,omitempty"`
	FsType      string   `json:"fs_type,omitempty"`
	Options     []string `json:"options,omitempty"`
	Chowner     []uint32 `json:"chowner,omitempty"` // array of two integers representing a tuple
	Chmode      uint32   `json:"chmode,omitempty"`
	// Lockhard also expects a "type" field here, which in our case is constant, so we provide it upon serialization below
}

// Adds the "type": "drive" to json
func (m *DriveMount) MarshalJSON() ([]byte, error) {
	type driveMountAlias DriveMount

	return json.Marshal(&struct {
		Type string `json:"type,omitempty"`
		*driveMountAlias
	}{
		Type:            "drive",
		driveMountAlias: (*driveMountAlias)(m),
	})
}

type BindMount struct {
	Source      string   `json:"source,omitempty"`
	Destination string   `json:"destination,omitempty"`
	Options     []string `json:"options,omitempty"`
}

func (m *BindMount) MarshalJSON() ([]byte, error) {
	type bindMountAlias BindMount

	return json.Marshal(&struct {
		Type string `json:"type,omitempty"`
		*bindMountAlias
	}{
		Type:           "bind",
		bindMountAlias: (*bindMountAlias)(m),
	})
}

type Capabilities struct {
	Ambient     []string `json:"ambient,omitempty"`
	Bounding    []string `json:"bounding,omitempty"`
	Effective   []string `json:"effective,omitempty"`
	Inheritable []string `json:"inheritable,omitempty"`
	Permitted   []string `json:"permitted,omitempty"`
}

type CgroupProfiles struct {
	Throttled   CgroupProfileConfig `json:"throttled"`
	Unthrottled CgroupProfileConfig `json:"unthrottled"`
}

type CgroupProfileConfig struct {
	CPULimit         float64 `json:"cpu_limit"`
	MemoryLimitBytes uint64  `json:"memory_limit_bytes"`
}

type ExecUser struct {
	UID *uint32 `json:"uid"`
	GID *uint32 `json:"gid"`
}

type ConfigureRequest struct {
	// domain to configure
	Domain         string        `json:"domain"`
	Mounts         []Mount       `json:"mounts,omitempty"`
	Capabilities   *Capabilities `json:"capabilities,omitempty"`
	SeccompFilters []string      `json:"seccomp_filters,omitempty"`
	// list of cgroup profiles available for the domain
	// cgroup profiles are set on start and thaw request. Start profile
	// if configured (as it can vary), thaw profile is always the same (throttled)
	CgroupProfiles *CgroupProfiles `json:"cgroup_profiles,omitempty"`
	// name of the cgroup profile to enforce at domain start
	StartProfile CgroupProfileName `json:"start_profile,omitempty"`
	// uid and gid of the user the spawned process runs as (w.r.t. the domain user namespace).
	// If nil, Supervisor will use the ExecUser specified in the domain configuration file
	ExecUser *ExecUser `json:"exec_user,omitempty"`
	// additional hooks to execute on domain start
	AdditionalStartHooks []Hook `json:"additional_start_hooks,omitempty"`
}

type EventsRequest struct {
	Domain string `json:"domain"`
}

type Event struct {
	Time  uint64    `json:"timestamp_millis"`
	Event EventData `json:"event"`
}

// EventData is a union type tagged by the "EventType"
// and "Cause" strings.
// you can use ProcessTermination() or EventLoss() to access
// the correct type of Event.
type EventData struct {
	EvType     string  `json:"type"`
	Domain     *string `json:"domain"`
	Name       *string `json:"name"`
	Cause      *string `json:"cause"`
	Signo      *int32  `json:"signo"`
	ExitStatus *int32  `json:"exit_status"`
	Size       *uint64 `json:"size"`
}

// returns nil if the event is not a EventLoss event
// otherwise returns how many events were lost due to
// backpressure (slow reader)
func (d EventData) EventLoss() *uint64 {
	return d.Size
}

// Returns a ProcessTermination struct that describe the process
// which terminated. Use Signaled() or Exited() to check whether
// the process terminated because of a signal or exited on its own
func (d EventData) ProcessTerminated() *ProcessTermination {
	if d.Signo != nil || d.ExitStatus != nil {
		return &ProcessTermination{
			Domain:     d.Domain,
			Name:       d.Name,
			Signo:      d.Signo,
			ExitStatus: d.ExitStatus,
		}
	}
	return nil
}

// Event signalling that a process exited
type ProcessTermination struct {
	Domain     *string
	Name       *string
	Signo      *int32
	ExitStatus *int32
}

// If not nil, the process was terminated by an unhandled signal.
// The returned value is the number of the signal that terminated the process
func (t ProcessTermination) Signaled() *int32 {
	return t.Signo
}

// It not nil, the process exited (as opposed to killed by a signal).
// The returned value is the exit_status returned by the process
func (t ProcessTermination) Exited() *int32 {
	return t.ExitStatus
}

func (t ProcessTermination) Success() bool {
	return t.ExitStatus != nil && *t.ExitStatus == 0
}

// Transform the process termination status in a string that
// is equal to what would be returned by golang exec.ExitError.Error()
// We used to rely on this format to report errors to customer (sigh)
// so we keep this for backwards compatibility
func (t ProcessTermination) String() string {
	if t.ExitStatus != nil {
		return fmt.Sprintf("exit status %d", *t.ExitStatus)
	}
	sig := syscall.Signal(*t.Signo)
	return fmt.Sprintf("signal: %s", sig.String())
}

type Hook struct {
	// Unique name identifying the hook
	Name string `json:"name"`
	// Path in the parent domain mount namespace that locates
	// the executable to run as the hook
	Path string `json:"path"`
	// Args for the hook
	Args []string `json:"args,omitempty"`
	// Map of ENV variables to set when running the hook
	Env *map[string]string `json:"envs,omitempty"`
}

type ExecRequest struct {
	// Identifier that Supervisor will assign to the spawned process.
	// The tuple (Domain,Name) must be unique. It is the caller's responsibility
	// to generate the unique name
	Name   string `json:"name"`
	Domain string `json:"domain"`
	// Path pointing to the exectuable file within the domain's root filesystem
	Path string   `json:"path"`
	Args []string `json:"args,omitempty"`
	// If nil, root of the domain
	Cwd          *string            `json:"cwd,omitempty"`
	Env          *map[string]string `json:"env,omitempty"`
	Logging      Logging            `json:"log_config"`
	StdoutWriter io.Writer          `json:"-"`
	StderrWriter io.Writer          `json:"-"`
	ExtraFiles   *[]*os.File        `json:"-"`
}

// Logging specifies where Supervisor should send Command's logs to
type Logging struct {
	Managed ManagedLogging `json:"managed"`
}

type ManagedLogging struct {
	Topic   ManagedLoggingTopic    `json:"topic"`
	Formats []ManagedLoggingFormat `json:"formats"`
}

type ManagedLoggingTopic string

const (
	RuntimeManagedLoggingTopic     ManagedLoggingTopic = "runtime"
	RtExtensionManagedLoggingTopic ManagedLoggingTopic = "runtime_extension"
)

type ManagedLoggingFormat string

const (
	LineBasedManagedLogging    ManagedLoggingFormat = "line"
	MessageBasedManagedLogging ManagedLoggingFormat = "message"
)

type ErrorKind string

const (
	// operation on an unkown entity (e.g., domain process)
	NoSuchEntity ErrorKind = "no_such_entity"
	// operation not allowed in the current state (e.g., tried to exec a proces in a domain which is not booted)
	InvalidState ErrorKind = "invalid_state"
	// Serialization or derserialization issue in the communication
	Serde ErrorKind = "serde"
	// Unhandled Supervisor server error
	Failure ErrorKind = "failure"
)

type SupervisorError struct {
	Kind    ErrorKind `json:"error_kind"`
	Message *string   `json:"message"`
}

func (e *SupervisorError) Error() string {
	return string(e.Kind)
}

// Send SIGETERM asynchrnously to a process
type TerminateRequest struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// Force terminate a process (SIGKILL)
// Block until process is exited or timeout
// Deadline needs to be in the future
type KillRequest struct {
	Name     string    `json:"name"`
	Domain   string    `json:"domain"`
	Deadline time.Time `json:"deadline"`
}

// Stop the domain.
type StopRequest struct {
	Domain   string    `json:"domain"`
	Deadline time.Time `json:"deadline"`
}

type StopResponse struct {
	CycleDeltaMetrics CycleDeltaMetrics `json:"cycle_delta_metrics"`
}

type FreezeRequest struct {
	Domain string `json:"domain"`
}

type FreezeResponse struct {
	CycleDeltaMetrics CycleDeltaMetrics `json:"cycle_delta_metrics"`
}

type MicrovmNetworkInterfaceMetrics struct {
	ReceivedBytes    uint64 `json:"received_bytes"`
	TransmittedBytes uint64 `json:"transmitted_bytes"`
}

type CycleDeltaMetrics struct {
	// CPU time (in nanoseconds) obtained by domain cgroup from cpuacct.usage
	// https://www.kernel.org/doc/Documentation/cgroup-v1/cpuacct.txt
	DomainCPURunNs uint64 `json:"domain_cpu_run_ns"`
	// time (in nanoseconds) for domain cycle
	DomainRunNs uint64 `json:"domain_run_ns"`
	// CPU delta time for service cgroup
	ServiceCPURunNs uint64 `json:"service_cpu_run_ns"`
	// Maximum memory used (in bytes) for domain
	DomainMaxMemoryUsageBytes uint64 `json:"domain_max_memory_usage_bytes"`
	// CPU delta time (in nanoseconds) obtained from /sys/fs/cgroup/cpu,cpuacct/cpuacct.usage
	MicrovmCPURunNs uint64 `json:"microvm_cpu_run_ns"`
	// Map with network interface name as key and network metrics as a value
	MicrovmNetworksBytes map[string]MicrovmNetworkInterfaceMetrics `json:"microvm_network_interfaces"`
	// time ( in nanoseconds ) for idle cpu time
	InvokeIdleCPURunNs uint64 `json:"idle_cpu_run_ns"`
}

type ThawRequest struct {
	Domain string `json:"domain"`
}
