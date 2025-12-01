// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"log/slog"
	"path"
	"path/filepath"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

const (
	ExtensionsDir = "/opt/extensions"
)

func ListExternalAgentPaths(fileutils utils.FileUtil, dir string, root string) []string {
	var agentPaths []string
	if !isCanonical(dir) || !isCanonical(root) {
		slog.Warn("Agents base paths are not absolute and in canonical form", "dir", dir, "root", root)
		return agentPaths
	}
	fullDir := path.Join(root, dir)
	files, err := fileutils.ReadDirectory(fullDir)
	if err != nil {
		if fileutils.IsNotExist(err) {
			slog.Info("The extension's directory does not exist, assuming no extensions to be loaded", "fullDir", fullDir)
		} else {

			slog.Error("Cannot list external agents", "err", err)
		}

		return agentPaths
	}

	for _, file := range files {
		if !file.IsDir() {

			p := path.Join("/", dir, file.Name())
			agentPaths = append(agentPaths, p)
		}
	}
	return agentPaths
}

func isCanonical(path string) bool {
	absPath, err := filepath.Abs(path)
	return err == nil && absPath == path
}
