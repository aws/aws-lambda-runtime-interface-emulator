// main entrypoint of init
// initial structure based upon /cmd/aws-lambda-rie/main.go
package main

import (
	"context"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rie"
	log "github.com/sirupsen/logrus"
)

type LsOpts struct {
	InteropPort         string
	RuntimeEndpoint     string
	RuntimeId           string
	AccountId           string
	InitTracingPort     string
	User                string
	CodeArchives        string
	HotReloadingPaths   []string
	FileWatcherStrategy string
	ChmodPaths          string
	LocalstackIP        string
	InitLogLevel        string
	EdgePort            string
	EnableXRayTelemetry string
	PostInvokeWaitMS    string
	MaxPayloadSize      string
}

func GetEnvOrDie(env string) string {
	result, found := os.LookupEnv(env)
	if !found {
		panic("Could not find environment variable for: " + env)
	}
	return result
}

func InitLsOpts() *LsOpts {
	return &LsOpts{
		// required
		RuntimeEndpoint: GetEnvOrDie("LOCALSTACK_RUNTIME_ENDPOINT"),
		RuntimeId:       GetEnvOrDie("LOCALSTACK_RUNTIME_ID"),
		AccountId:       GetenvWithDefault("LOCALSTACK_FUNCTION_ACCOUNT_ID", "000000000000"),
		// optional with default
		InteropPort:     GetenvWithDefault("LOCALSTACK_INTEROP_PORT", "9563"),
		InitTracingPort: GetenvWithDefault("LOCALSTACK_RUNTIME_TRACING_PORT", "9564"),
		User:            GetenvWithDefault("LOCALSTACK_USER", "sbx_user1051"),
		InitLogLevel:    GetenvWithDefault("LOCALSTACK_INIT_LOG_LEVEL", "warn"),
		EdgePort:        GetenvWithDefault("EDGE_PORT", "4566"),
		MaxPayloadSize:  GetenvWithDefault("LOCALSTACK_MAX_PAYLOAD_SIZE", "6291556"),
		// optional or empty
		CodeArchives:        os.Getenv("LOCALSTACK_CODE_ARCHIVES"),
		HotReloadingPaths:   strings.Split(GetenvWithDefault("LOCALSTACK_HOT_RELOADING_PATHS", ""), ","),
		FileWatcherStrategy: os.Getenv("LOCALSTACK_FILE_WATCHER_STRATEGY"),
		EnableXRayTelemetry: os.Getenv("LOCALSTACK_ENABLE_XRAY_TELEMETRY"),
		LocalstackIP:        os.Getenv("LOCALSTACK_HOSTNAME"),
		PostInvokeWaitMS:    os.Getenv("LOCALSTACK_POST_INVOKE_WAIT_MS"),
		ChmodPaths:          GetenvWithDefault("LOCALSTACK_CHMOD_PATHS", "[]"),
	}
}

// UnsetLsEnvs unsets environment variables specific to LocalStack to achieve better runtime parity with AWS
func UnsetLsEnvs() {
	unsetList := [...]string{
		// LocalStack internal
		"LOCALSTACK_RUNTIME_ENDPOINT",
		"LOCALSTACK_RUNTIME_ID",
		"LOCALSTACK_INTEROP_PORT",
		"LOCALSTACK_RUNTIME_TRACING_PORT",
		"LOCALSTACK_USER",
		"LOCALSTACK_CODE_ARCHIVES",
		"LOCALSTACK_HOT_RELOADING_PATHS",
		"LOCALSTACK_ENABLE_XRAY_TELEMETRY",
		"LOCALSTACK_INIT_LOG_LEVEL",
		"LOCALSTACK_POST_INVOKE_WAIT_MS",
		"LOCALSTACK_FUNCTION_ACCOUNT_ID",
		"LOCALSTACK_MAX_PAYLOAD_SIZE",
		"LOCALSTACK_CHMOD_PATHS",

		// Docker container ID
		"HOSTNAME",
		// User
		"HOME",
	}
	for _, envKey := range unsetList {
		if err := os.Unsetenv(envKey); err != nil {
			log.Warnln("Could not unset environment variable:", envKey, err)
		}
	}
}

func main() {
	// we're setting this to the same value as in the official RIE
	debug.SetGCPercent(33)

	// configuration parsing
	lsOpts := InitLsOpts()
	UnsetLsEnvs()

	// set up logging following the Logrus logging levels: https://github.com/sirupsen/logrus#level-logging
	log.SetReportCaller(true)
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-daemon-configuration.html
	xRayLogLevel := "info"
	switch lsOpts.InitLogLevel {
	case "trace":
		log.SetFormatter(&log.JSONFormatter{})
		log.SetLevel(log.TraceLevel)
		xRayLogLevel = "debug"
	case "debug":
		log.SetLevel(log.DebugLevel)
		xRayLogLevel = "debug"
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
		xRayLogLevel = "warn"
	case "error":
		log.SetLevel(log.ErrorLevel)
		xRayLogLevel = "error"
	case "fatal":
		log.SetLevel(log.FatalLevel)
		xRayLogLevel = "error"
	case "panic":
		log.SetLevel(log.PanicLevel)
		xRayLogLevel = "error"
	default:
		log.Fatal("Invalid value for LOCALSTACK_INIT_LOG_LEVEL")
	}

	// patch MaxPayloadSize
	payloadSize, err := strconv.Atoi(lsOpts.MaxPayloadSize)
	if err != nil {
		log.Panicln("Please specify a number for LOCALSTACK_MAX_PAYLOAD_SIZE")
	}
	interop.MaxPayloadSize = payloadSize

	// download code archive if env variable is set
	if err := DownloadCodeArchives(lsOpts.CodeArchives); err != nil {
		log.Fatal("Failed to download code archives: " + err.Error())
	}

	if err := AdaptFilesystemPermissions(lsOpts.ChmodPaths); err != nil {
		log.Warnln("Could not change file mode of code directories:", err)
	}

	// parse CLI args
	bootstrap, handler := getBootstrap(os.Args)

	// Switch to non-root user and drop root privileges
	if IsRootUser() && lsOpts.User != "" && lsOpts.User != "root" {
		uid := 993
		gid := 990
		AddUser(lsOpts.User, uid, gid)
		if err := os.Chown("/tmp", uid, gid); err != nil {
			log.Warnln("Could not change owner of directory /tmp:", err)
		}
		UserLogger().Debugln("Process running as root user.")
		err := DropPrivileges(lsOpts.User)
		if err != nil {
			log.Warnln("Could not drop root privileges.", err)
		} else {
			UserLogger().Debugln("Process running as non-root user.")
		}
	}

	// file watcher for hot-reloading
	fileWatcherContext, cancelFileWatcher := context.WithCancel(context.Background())

	logCollector := NewLogCollector()
	localStackLogsEgressApi := NewLocalStackLogsEgressAPI(logCollector)
	tracer := NewLocalStackTracer()

	// build sandbox
	sandbox := rapidcore.
		NewSandboxBuilder().
		//SetTracer(tracer).
		AddShutdownFunc(func() {
			log.Debugln("Stopping file watcher")
			cancelFileWatcher()
		}).
		SetExtensionsFlag(true).
		SetInitCachingFlag(true).
		SetLogsEgressAPI(localStackLogsEgressApi).
		SetTracer(tracer)

	// Corresponds to the 'AWS_LAMBDA_RUNTIME_API' environment variable.
	// We need to ensure the runtime server is up before the INIT phase,
	// but this envar is only set after the InitHandler is called.
	runtimeAPIAddress := "127.0.0.1:9001"
	sandbox.SetRuntimeAPIAddress(runtimeAPIAddress)

	// xray daemon
	endpoint := "http://" + lsOpts.LocalstackIP + ":" + lsOpts.EdgePort
	xrayConfig := initConfig(endpoint, xRayLogLevel)
	d := initDaemon(xrayConfig, lsOpts.EnableXRayTelemetry == "1")
	sandbox.AddShutdownFunc(func() {
		log.Debugln("Shutting down xray daemon")
		d.stop()
		log.Debugln("Flushing segments in xray daemon")
		d.close()
	})
	runDaemon(d) // async

	defaultInterop := sandbox.DefaultInteropServer()
	interopServer := NewCustomInteropServer(lsOpts, defaultInterop, logCollector)
	sandbox.SetInteropServer(interopServer)
	if len(handler) > 0 {
		sandbox.SetHandler(handler)
	}
	exitChan := make(chan struct{})
	sandbox.AddShutdownFunc(func() {
		exitChan <- struct{}{}
	})

	// initialize all flows and start runtime API
	sandboxContext, internalStateFn := sandbox.Create()
	// Populate our custom interop server
	interopServer.SetSandboxContext(sandboxContext)
	interopServer.SetInternalStateGetter(internalStateFn)

	// get timeout
	invokeTimeoutEnv := GetEnvOrDie("AWS_LAMBDA_FUNCTION_TIMEOUT") // TODO: collect all AWS_* env parsing
	invokeTimeoutSeconds, err := strconv.Atoi(invokeTimeoutEnv)
	if err != nil {
		log.Fatalln(err)
	}
	go RunHotReloadingListener(interopServer, lsOpts.HotReloadingPaths, fileWatcherContext, lsOpts.FileWatcherStrategy)

	log.Debugf("Awaiting initialization of runtime api at %s.", runtimeAPIAddress)
	// Fixes https://github.com/localstack/localstack/issues/12680
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := waitForRuntimeAPI(ctx, runtimeAPIAddress); err != nil {
		log.Fatalf("Lambda Runtime API server at %s did not come up in 30s, with error %s", runtimeAPIAddress, err.Error())
	}
	cancel()

	// start runtime init. It is important to start `InitHandler` synchronously because we need to ensure the
	// notification channels and status fields are properly initialized before `AwaitInitialized`
	log.Debugln("Starting runtime init.")
	rie.InitHandler(sandbox.LambdaInvokeAPI(), GetEnvOrDie("AWS_LAMBDA_FUNCTION_VERSION"), int64(invokeTimeoutSeconds), bootstrap) // TODO: replace this with a custom init

	log.Debugln("Awaiting initialization of runtime init.")
	if err := interopServer.delegate.AwaitInitialized(); err != nil {
		// Error cases: ErrInitDoneFailed or ErrInitResetReceived
		log.Errorln("Runtime init failed to initialize: " + err.Error() + ". Exiting.")
		// NOTE: Sending the error status to LocalStack is handled beforehand in the custom_interop.go through the
		// callback SendInitErrorResponse because it contains the correct error response payload.
		return
	}

	log.Debugln("Completed initialization of runtime init. Sending status ready to LocalStack.")
	if err := interopServer.localStackAdapter.SendStatus(Ready, []byte{}); err != nil {
		log.Fatalln("Failed to send status ready to LocalStack " + err.Error() + ". Exiting.")
	}

	<-exitChan
}
