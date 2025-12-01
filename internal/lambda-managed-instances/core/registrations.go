// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"log/slog"
	"os"
	"sync"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core/statejson"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type registrationServiceState int

const (
	registrationServiceOn registrationServiceState = iota
	registrationServiceOff
)

const MaxAgentsAllowed = 10

type Event string

const (
	ShutdownEvent Event = "SHUTDOWN"
)

var ErrRegistrationServiceOff = errors.New("ErrRegistrationServiceOff")

var ErrTooManyExtensions = errors.New("ErrTooManyExtensions")

func MapErrorToAgentInfoErrorType(err error) model.ErrorType {
	if errors.Is(err, os.ErrPermission) {
		return model.ErrorAgentPermissionDenied
	}
	if err == ErrTooManyExtensions {
		return model.ErrorAgentTooManyExtensions
	}
	return model.ErrorAgentExtensionLaunch
}

type AgentInfo struct {
	Name          string
	State         string
	Subscriptions []string
	ErrorType     model.ErrorType
}

type RegistrationService interface {
	CreateExternalAgent(agentName string) (*ExternalAgent, error)
	CreateInternalAgent(agentName string) (*InternalAgent, error)
	PreregisterRuntime(r *Runtime) error
	SetFunctionMetadata(metadata model.FunctionMetadata)
	GetFunctionMetadata() model.FunctionMetadata
	GetRuntime() *Runtime
	GetRegisteredAgentsSize() uint16
	FindExternalAgentByName(agentName string) (*ExternalAgent, bool)
	FindInternalAgentByName(agentName string) (*InternalAgent, bool)
	FindExternalAgentByID(agentID uuid.UUID) (*ExternalAgent, bool)
	FindInternalAgentByID(agentID uuid.UUID) (*InternalAgent, bool)
	TurnOff()
	InitFlow() InitFlowSynchronization
	GetInternalStateDescriptor(appCtx appctx.ApplicationContext) func() statejson.InternalStateDescription
	GetInternalAgents() []*InternalAgent
	GetExternalAgents() []*ExternalAgent
	GetSubscribedExternalAgents(eventType Event) []*ExternalAgent
	CountAgents() int
	Clear()
	AgentsInfo() []AgentInfo
	CancelFlows(err error)
}

type registrationServiceImpl struct {
	runtime          *Runtime
	internalAgents   AgentsMap[*InternalAgent]
	externalAgents   AgentsMap[*ExternalAgent]
	state            registrationServiceState
	mutex            *sync.Mutex
	initFlow         InitFlowSynchronization
	functionMetadata model.FunctionMetadata
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
		isd.FirstFatalError = string(fatalerror.ErrorType())
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

func (s *registrationServiceImpl) CreateExternalAgent(agentName string) (*ExternalAgent, error) {
	agent := NewExternalAgent(agentName, s.initFlow)

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

func (s *registrationServiceImpl) CreateInternalAgent(agentName string) (*InternalAgent, error) {
	agent := NewInternalAgent(agentName, s.initFlow)

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

func (s *registrationServiceImpl) PreregisterRuntime(r *Runtime) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.state != registrationServiceOn {
		return ErrRegistrationServiceOff
	}

	s.runtime = r

	return nil
}

func (s *registrationServiceImpl) SetFunctionMetadata(metadata model.FunctionMetadata) {
	s.functionMetadata = metadata
}

func (s *registrationServiceImpl) GetFunctionMetadata() model.FunctionMetadata {
	return s.functionMetadata
}

func (s *registrationServiceImpl) GetRuntime() *Runtime {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.runtime
}

func (s *registrationServiceImpl) GetRegisteredAgentsSize() uint16 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return uint16(s.externalAgents.Size()) + uint16(s.internalAgents.Size())
}

func (s *registrationServiceImpl) FindExternalAgentByName(name string) (agent *ExternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.externalAgents.FindByName(name); found {
		return agent, found
	}
	return agent, found
}

func (s *registrationServiceImpl) FindInternalAgentByName(name string) (agent *InternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.internalAgents.FindByName(name); found {
		return agent, found
	}
	return agent, found
}

func (s *registrationServiceImpl) FindExternalAgentByID(agentID uuid.UUID) (agent *ExternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.externalAgents.FindByID(agentID); found {
		return agent, found
	}
	return agent, found
}

func (s *registrationServiceImpl) FindInternalAgentByID(agentID uuid.UUID) (agent *InternalAgent, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if agent, found = s.internalAgents.FindByID(agentID); found {
		return agent, found
	}
	return agent, found
}

func (s *registrationServiceImpl) AgentsInfo() []AgentInfo {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	agentsInfo := []AgentInfo{}
	for _, agent := range s.GetExternalAgents() {
		agentsInfo = append(agentsInfo, AgentInfo{
			agent.Name(),
			agent.GetState().Name(),
			agent.SubscribedEvents(),
			agent.ErrorType(),
		})
	}

	for _, agent := range s.GetInternalAgents() {
		agentsInfo = append(agentsInfo, AgentInfo{
			agent.Name(),
			agent.GetState().Name(),
			[]string{},
			agent.ErrorType(),
		})
	}

	return agentsInfo
}

func (s *registrationServiceImpl) TurnOff() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.state = registrationServiceOff
}

func (s *registrationServiceImpl) CancelFlows(err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cancelOnce.Do(func() {
		slog.Debug("Canceling flows", "err", err)
		s.initFlow.CancelWithError(err)
	})
}

func NewRegistrationService(initFlow InitFlowSynchronization) RegistrationService {
	return &registrationServiceImpl{
		mutex:          &sync.Mutex{},
		state:          registrationServiceOn,
		internalAgents: NewAgentsMap[*InternalAgent](),
		externalAgents: NewAgentsMap[*ExternalAgent](),
		initFlow:       initFlow,
		cancelOnce:     sync.Once{},
	}
}
