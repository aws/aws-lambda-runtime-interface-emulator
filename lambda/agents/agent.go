// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"os"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// ListExternalAgentPaths return a list of external agents found in a given directory
func ListExternalAgentPaths(dir string, root string) []string {
	var agentPaths []string
	if !isCanonical(dir) || !isCanonical(root) {
		log.Warningf("Agents base paths are not absolute and in canonical form: %s, %s", dir, root)
		return agentPaths
	}
	fullDir := path.Join(root, dir)
	files, err := os.ReadDir(fullDir)

	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("The extension's directory %q does not exist, assuming no extensions to be loaded.", fullDir)
		} else {
			// TODO - Should this return an error rather than ignore failing to load?
			log.WithError(err).Error("Cannot list external agents")
		}

		return agentPaths
	}

	for _, file := range files {
		if !file.IsDir() {
			// The returned path is absolute wrt to `root`. This allows
			// to exec the agents in their own mount namespace
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
