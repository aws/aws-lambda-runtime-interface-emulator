// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func CreateTestSocketPair(t *testing.T) (fd [2]int) {
	domain, socketType := syscall.AF_UNIX, syscall.SOCK_DGRAM
	fds, err := syscall.Socketpair(domain, socketType, 0)
	if err != nil {
		t.Error("Could not create socketpair for testing: ", err)
	}

	return fds
}

func CreateTestLogFile(t *testing.T) *os.File {
	file, err := ioutil.TempFile(os.TempDir(), "rapid-unit-tests")
	assert.NoError(t, err, "error opening tmp log file for test")
	return file
}

type TestSocketsRapid struct {
	CtrlFd int
	CnslFd int
}

type TestSocketsSlicer struct {
	CtrlSock net.Conn
	CnslSock net.Conn
	CtrlFd   int
	CnslFd   int
}

func SetupTestSockets(t *testing.T) (TestSocketsRapid, TestSocketsSlicer) {
	ctrlFds := CreateTestSocketPair(t)
	testCtrlFd := os.NewFile(uintptr(ctrlFds[0]), "ctrlParent")

	cnslFds := CreateTestSocketPair(t)
	testCnslFd := os.NewFile(uintptr(cnslFds[0]), "cnslParent")

	ctrlSock, err := net.FileConn(testCtrlFd)
	assert.NoError(t, err, "failed to setup test socket")

	cnslSock, err := net.FileConn(testCnslFd)
	assert.NoError(t, err, "failed to setup test socket")

	rapidSockets := TestSocketsRapid{ctrlFds[1], cnslFds[1]}
	slicerSockets := TestSocketsSlicer{ctrlSock, cnslSock, ctrlFds[0], cnslFds[0]}
	return rapidSockets, slicerSockets
}

func SetupTestXRayUDPSocket(t *testing.T) net.PacketConn {
	pc, err := net.ListenPacket("udp", "localhost:0")
	assert.NoError(t, err, "failed to create udp listener for testing")
	return pc
}

func setTestDependenciesBinPath(t *testing.T) {
	var testDepsPath bytes.Buffer

	brazilPathCmd := exec.Command("brazil-path", "testrun.runtimefarm")
	brazilPathCmd.Stdout = &testDepsPath

	err := brazilPathCmd.Run()
	if err != nil {
		assert.Fail(t, "Could not run brazil-path to setup $PATH for test runtime")
	}

	testDepsBinPath := fmt.Sprintf("%s/bin", testDepsPath.String())

	err = os.Setenv("PATH", fmt.Sprintf("%s:%s", testDepsBinPath, os.Getenv("PATH")))
	if err != nil {
		assert.Fail(t, "Could not run brazil-path to setup $PATH for test runtime")
	}

	return
}

func SetupTestRuntime(t *testing.T, bootstrapScriptName string) (string, string) {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	resourcesDir := path.Join(base, "../testdata")

	bootstrap := path.Join(resourcesDir, bootstrapScriptName)
	taskRoot := filepath.Dir(bootstrap)

	setTestDependenciesBinPath(t)
	err := os.Setenv("LAMBDA_TASK_ROOT", taskRoot)
	assert.NoError(t, err)

	return bootstrap, taskRoot
}
