// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

const (
	InitInsideInitPhase   interop.InitPhase = "init"
	InitInsideInvokePhase interop.InitPhase = "invoke"
)

func initPhaseFromLifecyclePhase(phase interop.LifecyclePhase) (interop.InitPhase, error) {
	switch phase {
	case interop.LifecyclePhaseInit:
		return InitInsideInitPhase, nil
	case interop.LifecyclePhaseInvoke:
		return InitInsideInvokePhase, nil
	default:
		return interop.InitPhase(""), fmt.Errorf("unexpected lifecycle phase: %v", phase)
	}
}

func calculateDurationInMillis(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

type NoOpEventsAPI struct{}

func (s *NoOpEventsAPI) SetCurrentRequestID(interop.InvokeID) {}

func (s *NoOpEventsAPI) SendInitStart(interop.InitStartData) error { return nil }

func (s *NoOpEventsAPI) SendInitRuntimeDone(interop.InitRuntimeDoneData) error { return nil }

func (s *NoOpEventsAPI) SendInitReport(interop.InitReportData) error { return nil }

func (s *NoOpEventsAPI) SendExtensionInit(interop.ExtensionInitData) error { return nil }

func (s *NoOpEventsAPI) SendImageError(interop.ImageErrorLogData) {}

func (s *NoOpEventsAPI) SendInvokeStart(interop.InvokeStartData) error { return nil }

func (s *NoOpEventsAPI) SendReport(interop.ReportData) error { return nil }

func (s *NoOpEventsAPI) SendInternalXRayErrorCause(interop.InternalXRayErrorCauseData) error {
	return nil
}

func (s *NoOpEventsAPI) Flush() {

}

type Event struct {
	Time   string          `json:"time"`
	Type   string          `json:"type"`
	Record json.RawMessage `json:"record"`
}

func FormatImageError(errLog interop.ImageErrorLogData) string {
	switch errLog.ExecError.Type {
	case model.InvalidTaskConfig:
		return fmt.Sprintf("IMAGE\tInvalid task config: %s", errLog.ExecError.Err)
	case model.InvalidEntrypoint:
		return fmt.Sprintf("IMAGE\tLaunch error: %s\tEntrypoint: %s\tCmd: [%s]\tWorkingDir: [%s]",
			errLog.ExecError.Err,
			errLog.ExecConfig.Cmd[0],
			strings.Join(errLog.ExecConfig.Cmd[1:], ","),
			errLog.ExecConfig.WorkingDir)
	case model.InvalidWorkingDir:
		return fmt.Sprintf("IMAGE\tLaunch error: %s\tEntrypoint: %s\tCmd: [%s]\tWorkingDir: [%s]",
			errLog.ExecError.Err,
			errLog.ExecConfig.Cmd[0],
			strings.Join(errLog.ExecConfig.Cmd[1:], ","),
			errLog.ExecConfig.WorkingDir)
	default:

		invariant.Violatef("Invalid runtime exec error type: %d", int(errLog.ExecError.Type))
		return ""
	}
}
