// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package run

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/local"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

func Run() {
	server, rieHandler, _, err := internal.Run(local.NewProcessSupervisor(local.WithLowerPriorities(false)), os.Args, utils.NewFileUtil(), make(chan os.Signal, 1))
	if err != nil {
		slog.Error("rie failed", "err", err)
		os.Exit(1)
	}

	if err := rieHandler.Init(); err != nil {
		slog.Warn("INIT failed", "err", err)

	}

	<-server.Done()
	if err := server.Err(); err != nil {
		slog.Warn("rie server stopped", "err", err)
		os.Exit(1)
	}
}
