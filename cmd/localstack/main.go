// main entrypoint of init
// initial structure based upon /cmd/aws-lambda-rie/main.go
package main

import (
	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/rapidcore"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/debug"
)

type LsOpts struct {
	InteropPort     string
	RuntimeEndpoint string
	RuntimeId       string
	InitTracingPort string
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
		RuntimeEndpoint: GetEnvOrDie("LOCALSTACK_RUNTIME_ENDPOINT"),
		RuntimeId:       GetEnvOrDie("LOCALSTACK_RUNTIME_ID"),
		// optional with default
		InteropPort:     GetenvWithDefault("LOCALSTACK_INTEROP_PORT", "9563"),
		InitTracingPort: GetenvWithDefault("LOCALSTACK_RUNTIME_TRACING_PORT", "9564"),
	}
}

func main() {
	// we're setting this to the same value as in the official RIE
	debug.SetGCPercent(33)

	lsOpts := InitLsOpts()

	// set up logging (logrus)
	//log.SetFormatter(&log.JSONFormatter{})
	//log.SetLevel(log.TraceLevel)
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	// parse CLI args
	opts, args := getCLIArgs()
	bootstrap, handler := getBootstrap(args, opts)
	logCollector := NewLogCollector()
	sandbox := rapidcore.
		NewSandboxBuilder(bootstrap).
		AddShutdownFunc(func() { os.Exit(0) }).
		SetExtensionsFlag(true).
		SetInitCachingFlag(true).
		SetTailLogOutput(logCollector)

	defaultInterop := sandbox.InteropServer()
	sandbox.SetInteropServer(NewCustomInteropServer(lsOpts, defaultInterop, logCollector))
	if len(handler) > 0 {
		sandbox.SetHandler(handler)
	}

	// initialize all flows and start runtime API
	go sandbox.Create()

	// start runtime init
	go InitHandler(sandbox, GetEnvOrDie("AWS_LAMBDA_FUNCTION_VERSION"), 30) // TODO: replace this with a custom init

	// TODO: make the tracing server optional
	// start blocking with the tracing server
	err := http.ListenAndServe("0.0.0.0:"+lsOpts.InitTracingPort, http.DefaultServeMux)
	if err != nil {
		log.Fatal("Failed to start debug server")
	}

}
