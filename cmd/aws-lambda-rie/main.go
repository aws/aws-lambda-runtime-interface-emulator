// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/run"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapidcore/env"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rie"
)

func main() {
	if _, ok := os.LookupEnv(env.AWS_LAMBDA_MAX_CONCURRENCY); ok {
		run.Run()
		return
	}
	rie.Run()
}
