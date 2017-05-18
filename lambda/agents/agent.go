// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// AgentProcess is the common interface exposed by both internal and external agent processes
type AgentProcess interface {
	Name() string
}

// ExternalAgentProcess represents an external agent process
type ExternalAgentProcess struct {
	cmd *exec.Cmd
}

// NewExternalAgentProcess returns a new external agent process
func NewExternalAgentProcess(path string, env []string, logWriter io.Writer) ExternalAgentProcess {
	command := exec.Command(path)
	command.Env = env

	w := NewNewlineSplitWriter(logWriter)
	command.Stdout = w
	command.Stderr = w
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return ExternalAgentProcess{
		cmd: command,
	}
}

// Name returns the name of the agent
// For external agents is the executable name
func (a *ExternalAgentProcess) Name() string {
	return path.Base(a.cmd.Path)
}

func (a *ExternalAgentProcess) Pid() int {
	return a.cmd.Process.Pid
}

// Start starts an external agent process
func (a *ExternalAgentProcess) Start() error {
	return a.cmd.Start()
}

// Wait waits for the external agent process to exit
func (a *ExternalAgentProcess) Wait() error {
	return a.cmd.Wait()
}

// String is used to print values passed as an operand to any format that accepts a string or to an unformatted printer such as Print.
func (a *ExternalAgentProcess) String() string {
	return fmt.Sprintf("%s (%s)", a.Name(), a.cmd.Path)
}

// ListExternalAgentPaths return a list of external agents found in a given directory
func ListExternalAgentPaths(root string) []string {
	var agentPaths []string
	files, err := ioutil.ReadDir(root)
	if err != nil {
		log.WithError(err).Warning("Cannot list external agents")
		return agentPaths
	}
	for _, file := range files {
		if !file.IsDir() {
			agentPaths = append(agentPaths, path.Join(root, file.Name()))
		}
	}
	return agentPaths
}
