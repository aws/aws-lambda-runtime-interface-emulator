// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
// LOCALSTACK CHANGES 2022-03-10: modified/collected file from /cmd/aws-lambda-rie/* into this util
// LOCALSTACK CHANGES 2022-03-10: minor refactoring of PrintEndReports
// LOCALSTACK CHANGES 2023-10-06: reflect getBootstrap and InitHandler API updates

package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore/env"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	optBootstrap     = "/opt/bootstrap"
	runtimeBootstrap = "/var/runtime/bootstrap"
)

func isBootstrapFileExist(filePath string) bool {
	file, err := os.Stat(filePath)
	return !os.IsNotExist(err) && !file.IsDir()
}

func getBootstrap(args []string) (interop.Bootstrap, string) {
	var bootstrapLookupCmd []string
	var handler string
	currentWorkingDir := "/var/task" // default value

	if len(args) <= 1 {
		// set default value to /var/task/bootstrap, but switch to the other options if it doesn't exist
		bootstrapLookupCmd = []string{
			fmt.Sprintf("%s/bootstrap", currentWorkingDir),
		}

		if !isBootstrapFileExist(bootstrapLookupCmd[0]) {
			var bootstrapCmdCandidates = []string{
				optBootstrap,
				runtimeBootstrap,
			}

			for i, bootstrapCandidate := range bootstrapCmdCandidates {
				if isBootstrapFileExist(bootstrapCandidate) {
					bootstrapLookupCmd = []string{bootstrapCmdCandidates[i]}
					break
				}
			}
		}

		// handler is used later to set an env var for Lambda Image support
		handler = ""
	} else if len(args) > 1 {

		bootstrapLookupCmd = args[1:]

		if cwd, err := os.Getwd(); err == nil {
			currentWorkingDir = cwd
		}

		if len(args) > 2 {
			// Assume last arg is the handler
			handler = args[len(args)-1]
		}

		log.Infof("exec '%s' (cwd=%s, handler=%s)", args[1], currentWorkingDir, handler)

	} else {
		log.Panic("insufficient arguments: bootstrap not provided")
	}

	err := unix.Access(bootstrapLookupCmd[0], unix.X_OK)
	if err != nil {
		log.Debug("Bootstrap not executable, setting permissions to 0755...", bootstrapLookupCmd[0])
		err = os.Chmod(bootstrapLookupCmd[0], 0755)
		if err != nil {
			log.Warn("Error setting bootstrap to 0755 permissions: ", bootstrapLookupCmd[0], err)
		}
	}

	return NewSimpleBootstrap(bootstrapLookupCmd, currentWorkingDir), handler
}

func PrintEndReports(invokeId string, initDuration string, memorySize string, invokeStart time.Time, timeoutDuration time.Duration, w io.Writer) {
	// Calculate invoke duration
	invokeDuration := math.Min(float64(time.Now().Sub(invokeStart).Nanoseconds()),
		float64(timeoutDuration.Nanoseconds())) / float64(time.Millisecond)

	_, _ = fmt.Fprintln(w, "END RequestId: "+invokeId)
	// We set the Max Memory Used and Memory Size to be the same (whatever it is set to) since there is
	// not a clean way to get this information from rapidcore
	_, _ = fmt.Fprintf(w,
		"REPORT RequestId: %s\t"+
			initDuration+
			"Duration: %.2f ms\t"+
			"Billed Duration: %.f ms\t"+
			"Memory Size: %s MB\t"+
			"Max Memory Used: %s MB\t\n",
		invokeId, invokeDuration, math.Ceil(invokeDuration), memorySize, memorySize)
}

type Sandbox interface {
	Init(i *interop.Init, invokeTimeoutMs int64)
	Invoke(responseWriter http.ResponseWriter, invoke *interop.Invoke) error
}

// GetenvWithDefault returns the value of the environment variable key or the defaultValue if key is not set
func GetenvWithDefault(key string, defaultValue string) string {
	envValue, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}

	return envValue
}

func resetListener(changeChannel <-chan bool, server *CustomInteropServer) {
	for {
		_, more := <-changeChannel
		if !more {
			return
		}
		log.Println("Resetting environment...")
		_, err := server.Reset("HotReload", 2000)
		if err != nil {
			log.Warnln("Error resetting server: ", err)
		}
	}

}

func RunHotReloadingListener(server *CustomInteropServer, targetPaths []string, ctx context.Context, fileWatcherStrategy string) {
	if len(targetPaths) == 1 && targetPaths[0] == "" {
		log.Debugln("Hot reloading disabled.")
		return
	}
	defaultDebouncingDuration := 500 * time.Millisecond
	log.Infoln("Hot reloading enabled, starting filewatcher.", targetPaths)
	changeListener, err := NewChangeListener(defaultDebouncingDuration, fileWatcherStrategy)
	if err != nil {
		log.Errorln("Hot reloading disabled due to change listener error.", err)
		return
	}
	defer changeListener.Close()
	go changeListener.Start()
	changeListener.AddTargetPaths(targetPaths)
	go resetListener(changeListener.debouncedChannel, server)

	<-ctx.Done()
	log.Infoln("Closing down filewatcher.")

}

func getSubFolders(dirPath string) []string {
	var subfolders []string
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			subfolders = append(subfolders, path)
		}
		return err
	})
	if err != nil {
		log.Errorln("Error listing directory contents: ", err)
		return subfolders
	}
	return subfolders
}

func getSubFoldersInList(prefix string, pathList []string) (oldFolders []string, newFolders []string) {
	for _, pathItem := range pathList {
		if strings.HasPrefix(pathItem, prefix) {
			oldFolders = append(oldFolders, pathItem)
		} else {
			newFolders = append(newFolders, pathItem)
		}
	}
	return
}

func InitHandler(sandbox Sandbox, functionVersion string, timeout int64, bs interop.Bootstrap, accountId string) (time.Time, time.Time) {
	additionalFunctionEnvironmentVariables := map[string]string{}

	// Add default Env Vars if they were not defined. This is a required otherwise 1p Python2.7, Python3.6, and
	// possibly others pre runtime API runtimes will fail. This will be overwritten if they are defined on the system.
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_LOG_GROUP_NAME"] = "/aws/lambda/Functions"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_LOG_STREAM_NAME"] = "$LATEST"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_FUNCTION_VERSION"] = "$LATEST"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_FUNCTION_MEMORY_SIZE"] = "3008"
	additionalFunctionEnvironmentVariables["AWS_LAMBDA_FUNCTION_NAME"] = "test_function"

	// Forward Env Vars from the running system (container) to what the function can view. Without this, Env Vars will
	// not be viewable when the function runs.
	for _, env := range os.Environ() {
		// Split the env into by the first "=". This will account for if the env var's value has a '=' in it
		envVar := strings.SplitN(env, "=", 2)
		additionalFunctionEnvironmentVariables[envVar[0]] = envVar[1]
	}

	initStart := time.Now()
	// pass to rapid
	sandbox.Init(&interop.Init{
		Handler:           GetenvWithDefault("AWS_LAMBDA_FUNCTION_HANDLER", os.Getenv("_HANDLER")),
		AwsKey:            os.Getenv("AWS_ACCESS_KEY_ID"),
		AwsSecret:         os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AwsSession:        os.Getenv("AWS_SESSION_TOKEN"),
		AccountID:         accountId,
		XRayDaemonAddress: GetenvWithDefault("AWS_XRAY_DAEMON_ADDRESS", "127.0.0.1:2000"),
		FunctionName:      GetenvWithDefault("AWS_LAMBDA_FUNCTION_NAME", "test_function"),
		FunctionVersion:   functionVersion,

		// TODO: Implement runtime management controls
		// https://aws.amazon.com/blogs/compute/introducing-aws-lambda-runtime-management-controls/
		RuntimeInfo: interop.RuntimeInfo{
			ImageJSON: "{}",
			Arn:       "",
			Version:   ""},
		CustomerEnvironmentVariables: additionalFunctionEnvironmentVariables,
		SandboxType:                  interop.SandboxClassic,
		Bootstrap:                    bs,
		EnvironmentVariables:         env.NewEnvironment(),
	}, timeout*1000)
	initEnd := time.Now()
	return initStart, initEnd
}
