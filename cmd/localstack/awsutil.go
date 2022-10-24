// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
// LOCALSTACK CHANGES 2022-03-10: modified/collected file from /cmd/aws-lambda-rie/* into this util
// LOCALSTACK CHANGES 2022-03-10: minor refactoring of PrintEndReports

package main

import (
	"archive/zip"
	"fmt"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	optBootstrap     = "/opt/bootstrap"
	runtimeBootstrap = "/var/runtime/bootstrap"
	taskFolder       = "/var/task"
)

type options struct {
	LogLevel           string `long:"log-level" default:"info" description:"log level"`
	InitCachingEnabled bool   `long:"enable-init-caching" description:"Enable support for Init Caching"`
}

func getCLIArgs() (options, []string) {
	var opts options
	parser := flags.NewParser(&opts, flags.IgnoreUnknown)
	args, err := parser.ParseArgs(os.Args)

	if err != nil {
		log.WithError(err).Fatal("Failed to parse command line arguments:", os.Args)
	}

	return opts, args
}

func isBootstrapFileExist(filePath string) bool {
	file, err := os.Stat(filePath)
	return !os.IsNotExist(err) && !file.IsDir()
}

func getBootstrap(args []string, opts options) (*rapidcore.Bootstrap, string) {
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

	return rapidcore.NewBootstrapSingleCmd(bootstrapLookupCmd, currentWorkingDir), handler
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

func GetenvWithDefault(key string, defaultValue string) string {
	envValue := os.Getenv(key)

	if envValue == "" {
		return defaultValue
	}

	return envValue
}

func DownloadCodeArchive(url string) {
	// download and unzip code archive, if url is given
	if url == "" {
		return
	}
	log.Infoln("Downloading code archive")
	// create tmp directory
	tmpDir := os.TempDir()
	// download code archive into tmp directory
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	tmp_file_path := path.Join(tmpDir, "code-archive.zip")
	tmp_file, err := os.OpenFile(tmp_file_path, os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(tmp_file, res.Body)
	if err != nil {
		log.Fatal(err)
	}
	err = tmp_file.Close()
	if err != nil {
		log.Fatal(err)
	}
	// unzip into /var/task
	log.Infoln("Unzipping code archive")
	r, err := zip.OpenReader(tmp_file_path)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		target_file_name := path.Join(taskFolder, f.Name)
		if f.FileInfo().IsDir() {
			err = os.MkdirAll(target_file_name, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target_file_name), os.ModePerm); err != nil {
			panic(err)
		}
		target_file, err := os.OpenFile(target_file_name, os.O_WRONLY|os.O_CREATE, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(target_file, rc)
		if err != nil {
			log.Fatal(err)
		}
		target_file.Close()
		rc.Close()
	}

}

func InitHandler(sandbox Sandbox, functionVersion string, timeout int64) (time.Time, time.Time) {
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
		CorrelationID:     "initCorrelationID",
		AwsKey:            os.Getenv("AWS_ACCESS_KEY_ID"),
		AwsSecret:         os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AwsSession:        os.Getenv("AWS_SESSION_TOKEN"),
		XRayDaemonAddress: "0.0.0.0:0", // TODO
		FunctionName:      GetenvWithDefault("AWS_LAMBDA_FUNCTION_NAME", "test_function"),
		FunctionVersion:   functionVersion,

		CustomerEnvironmentVariables: additionalFunctionEnvironmentVariables,
	}, timeout*1000)
	initEnd := time.Now()
	return initStart, initEnd
}
