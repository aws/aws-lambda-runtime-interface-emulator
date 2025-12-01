// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type FileUtil interface {
	ReadDirectory(dirPath string) ([]os.DirEntry, error)
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
}

func NewFileUtil() FileUtil {
	return &fileUtil{}
}

type fileUtil struct{}

func (l *fileUtil) ReadDirectory(dirPath string) ([]os.DirEntry, error) {
	return os.ReadDir(dirPath)
}

func (l *fileUtil) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (l *fileUtil) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func FixTmpDir(dir string, uid, gid int) error {
	if err := os.Chown(dir, uid, gid); err != nil {
		return fmt.Errorf("could not chown %s folder: %w", dir, err)
	}
	if err := os.Chmod(dir, fs.ModeDir|fs.ModeSticky|fs.ModePerm); err != nil {
		return fmt.Errorf("could not chmod %s folder: %w", dir, err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "lost+found")); err != nil {
		return fmt.Errorf("could not remove %s/lost+found folder: %w", dir, err)
	}

	return nil
}
