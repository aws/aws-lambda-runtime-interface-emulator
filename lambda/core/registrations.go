// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"os"
	"sync"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/interop"

	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

type registrationServiceState int

const (
	registrationServiceOn registrationServiceState = iota
	registrationServiceOff
)

const MaxAgentsAllowed = 10

// Event represents a platform event which agent can subscribe to
type Event string

const (
	// InvokeEvent is dispatched when INVOKE happens
	InvokeEvent Event = "INVOKE"
	// ShutdownEvent is dispatched when SHUTDOWN or RESET happen
	ShutdownEvent Event = "SHUTDOWN"
)

// ErrRegistrationServiceOff returned on attempt to register after registration service has been turned off.
var ErrRegistrationServiceOff = errors.New("ErrRegistrationServiceOff")

// ErrTooManyExtensions means MaxAgentsAllowed limit is exceeded
var ErrTooManyExtensions = errors.New("ErrTooManyExtensions")

type AgentInfoErrorType string

const (
	PermissionDenied  AgentInfoErrorType = "PermissionDenied"
	TooManyExtensions AgentInfoErrorType = "TooManyExtensions"
	UnknownError      AgentInfoErrorType = "UnknownError"
)

func MapErrorToAgentInfoErrorType(err error) AgentInfoErrorType {
	if os.IsPermission(err) {
		return PermissionDenied
	}
	if err == ErrTooManyExtensions {
		return TooManyExtensions
	}
	return UnknownError
}

// AgentInfo holds information about an agent renderable in customer logs
type AgentInfo struct {
	Name          string
	State         string
	Subscriptions []string
	ErrorType     string
}

// FunctionMetadata holds static information regarding the function (Name, Version, Handler)
type FunctionMetadata struct {
	AccountID         string
	FunctionName      string
	FunctionVersion   string
	InstanceMaxMemory uint64
	Handler           string
	RuntimeInfo       interop.RuntimeInfo
}

// RegistrationService keeps track of registered parties, including external agents, threads, and runtime.
type RegistrationService interface {
	CreateExternalAgent(agentName string) (*ExternalAgent, error)
	CreateInternalAgent(agentName string) (*InternalAgent, error)
	PreregisterRuntime(r *Runtime) error
	SetFunctionMetadata(metadata FunctionMetadata)
	GetFunctionMetadata() FunctionMetadata
	GetRuntime() *Runtime
	GetRegisteredAgentsSize() uint16
	FindExternalAgentByName(agentName string) (*ExternalAgent, bool)
	FindInternalAgentByName(agentName string) (*InternalAgent, bool)
	FindExternalAgentByID(agentID uuid.UUID) (*ExternalAgent, bool)
	FindInternalAgentByID(agentID uuid.UUID) (*InternalAgent, bool)
	TurnOff()
	InitFlow() InitFlowSynchronization
	GetInternalStateDescriptor(appCtx appctx.ApplicationContext) func() statejson.InternalStateDescription
	GetExternalAgents() []*ExternalAgent
	GetSubscribedExternalAgents(eventType Event) []*ExternalAgent
	GetSubscribedInternalAgents(eventType Event) []*InternalAgent
	CountAgents() int
	Clear()
	AgentsInfo() []AgentInfo
	CancelFlows(err error)
}

type registrationServiceImpl struct {
	runtime          *Runtime
	internalAgents   InternalAgentsMap
	externalAgents   ExternalAgentsMap
	state            registrationServiceState
	mutex            *sync.Mutex
	initFlow         InitFlowSynchronization
	invokeFlow       InvokeFlowSynchronization
	functionMetadata FunctionMetadata
	cancelOnce       sync.Once
}

func (s *registrationServiceImpl) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.runtime = nil
	s.internalAgents.Clear()
	s.externalAgents.Clear()
	s.state = registrationServiceOn
	s.cancelOnce = sync.Once{}
}

func (s *registrationServiceImpl) InitFlow() InitFlowSynchronization {
	return s.initFlow
}

// GetInternalStateDescriptor returns function that returns internal state description for debugging purposes
func (s *registrationServiceImpl) GetInternalStateDescriptor(appCtx appctx.ApplicationContext) func() statejson.InternalStateDescription {
	return func() statejson.InternalStateDescription {
		return s.getInternalStateDescription(appCtx)
	}
}

func (s *registrationServiceImpl) getInternalStateDescription(appCtx appctx.ApplicationContext) statejson.InternalStateDescription {
	isd := statejson.InternalStateDescription{
		Extensions: []statejson.ExtensionDescription{},
	}

	if s.runtime != nil {
		// we use pointer here so that 'runtime' json field is nil if runtime is not set (as opposed to filled with default values)
		rtdesc := s.runtime.GetRuntimeDescription()
		isd.Runtime = &rtdesc
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.internalAgents.Visit(func(agent *InternalAgent) {
		isd.Extensions = append(isd.Extensions, agent.GetAgentDescription())
	})

	s.externalAgents.Visit(func(agent *ExternalAgent) {
		isd.Extensions = append(isd.Extensions, agent.GetAgentDescription())
	})

	if fatalerror, found := appctx.LoadFirstFatalError(appCtx); found {
		isd.FirstFatalError = string(fatalerror)
	}

	return isd
}

func (s *registrationServiceImpl) CountAgents() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.countAgentsUnsafe()
}

func (s *registrationServiceImpl) countAgentsUnsafe() int {
	res := 0
	s.externalAgents.Visit(func(a *ExternalAgent) {
		res++
	})
	s.internalAgents.Visit(func(a *InternalAgent) {
		res++
	})
	return res
}

func (s *registrationServiceImpl) GetExternalAgents() []*ExternalAgent {
	agents := []*ExternalAgent{}
	s.externalAgents.Visit(func(a *ExternalAgent) {
		agents = append(agents, a)
	})
	return agents
}

func (s *registrationServiceImpl) GetInternalAgents() []*InternalAgent {
	agents := []*InternalAgent{}
	s.internalAgents.Visit(func(a *InternalAgent) {
		agents = append(agents, a)
	})
	return agents
}

func (s *registrationServiceImpl) GetSubscribedExternalAgents(eventType Event) []*ExternalAgent {
	agents := []*ExternalAgent{}
	s.externalAgents.Visit(func(a *ExternalAgent) {
		if a.IsSubscribed(eventType) {
			agents = append(agents, a)
		}
	})
	return agents
}

func (s *registrationServiceImpl) GetSubscribedInternalAgents(eventType Event) []*InternalAgent {
	agents := []*InternalAgent{}
	s.internalAgents.Visit(func(a *InternalAgent) {
		if a.IsSubscribed(eventType) {
			agents = append(agents, a)
		}
	})
	return agents
}

// CreateExternalAgent creates agent in agent collection
func (s *registrationServiceImpl) CreateExternalAgent(agentName string) (*ExternalAgent, error) {
	agent := NewExternalAgent(agentName, s.initFlow, s.invokeFlow)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.state != registrationServiceOn {
		return nil, ErrRegistrationServiceOff
	}

	if _, found := s.internalAgents.FindByName(agentName); found {
		return nil, ErrAgentNameCollision
	}

	if err := s.externalAgents.Insert(agent); err != nil {
		return nil, err
	}

	return agent, nil
}

// CreateInternalAgent creates agent in agent collection
func (s *registrationServiceImpl) CreateInternalAgent(agentName string) (*InternalAgent, error) {
	agent := NewInternalAgent(agentName, s.initFlow, s.invokeFlow)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.state != registrationServiceOn {
		return nil, ErrRegistrationServiceOff
	}

	if s.countAgentsUnsafe() >= MaxAgentsAllowed {
		return nil, ErrTooManyExtensions
	}

	if _, found := s.externalAgents.FindByName(agentName); found {
		return nil, ErrAgentNameCollision
	}

	if err := s.internalAgents.Insert(agent); err != nil {
		return nil, err
	}

	return agent, nil
}

// PreregisterRuntime allows to preregister a runtime.
func (s *registrationServiceImpl) PreregisterRuntime(r *Runtime) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.state != registrationServiceOn {
		return ErrRegistrationServiceOff
	}

	s.runtime = r

	// RUNTIME IS NOT PART OF SUBSCRIPTIONS

	return nil
}

// SetFunctionMetadata sets the static function metadata object
func (s *registrationServiceImpl) SetFunctionMetadata(metadata FunctionMetadata) {
	s.functionMetadata = metadata
}

// GetFunctionMetadata returns the static function metadata object
func (s *registrationServiceImpl) GetFunctionMetadata() FunctionMetadata {
	return s.functionMetadata
}

// GetRuntime retrieves runtime.
func (s *registrationServiceImpl) GetRuntime() *Runtime {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.runtime
}

// GetRegisteredAgentsSize retrieves the number of agents registered with the platform.
func (s *registrationServiceImpl) GetRegisteredAgentsSize() uint16 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return uint16(s.externalAgents.Size()) + uint16(s.internalAgents.Size())
}

// FindExternalAgentByName
func (s *registrationServiceImpl) FindExternalAgentByName(name string) (agent *ExternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.externalAgents.FindByName(name); found {
		return
	}
	return
}

// FindInternalAgentByName
func (s *registrationServiceImpl) FindInternalAgentByName(name string) (agent *InternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.internalAgents.FindByName(name); found {
		return
	}
	return
}

// FindExternalAgentByID
func (s *registrationServiceImpl) FindExternalAgentByID(agentID uuid.UUID) (agent *ExternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.externalAgents.FindByID(agentID); found {
		return
	}
	return
}

// FindInternalAgentByID
func (s *registrationServiceImpl) FindInternalAgentByID(agentID uuid.UUID) (agent *InternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.internalAgents.FindByID(agentID); found {
		return
	}
	return
}

// ReportAgentsInfo returns information about all agents
func (s *registrationServiceImpl) AgentsInfo() []AgentInfo {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	agentsInfo := []AgentInfo{}
	for _, agent := range s.GetExternalAgents() {
		agentsInfo = append(agentsInfo, AgentInfo{
			agent.Name,
			agent.GetState().Name(),
			agent.SubscribedEvents(),
			agent.ErrorType(),
		})
	}

	for _, agent := range s.GetInternalAgents() {
		agentsInfo = append(agentsInfo, AgentInfo{
			agent.Name,
			agent.GetState().Name(),
			agent.SubscribedEvents(),
			agent.ErrorType(),
		})
	}

	return agentsInfo
}

// TurnOff turns off registration service.
func (s *registrationServiceImpl) TurnOff() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.state = registrationServiceOff
}

// CancelFlows cancels init and invoke flows with error.
func (s *registrationServiceImpl) CancelFlows(err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// The following block protects us from overwriting the error
	// which was first used to cancel flows.
	s.cancelOnce.Do(func() {
		log.Debugf("Canceling flows: %s", err)
		s.initFlow.CancelWithError(err)
		s.invokeFlow.CancelWithError(err)
	})
}

// NewRegistrationService returns new RegistrationService instance.
func NewRegistrationService(initFlow InitFlowSynchronization, invokeFlow InvokeFlowSynchronization) RegistrationService {
	return &registrationServiceImpl{
		mutex:          &sync.Mutex{},
		state:          registrationServiceOn,
		internalAgents: NewInternalAgentsMap(),
		externalAgents: NewExternalAgentsMap(),
		initFlow:       initFlow,
		invokeFlow:     invokeFlow,
		cancelOnce:     sync.Once{},
	}
}
