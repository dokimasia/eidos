// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The modes of the directories and the files that the harness writes. The
// scratch project is private to the user who runs the check.
const (
	dirMode  = 0o700
	fileMode = 0o600
)

// Scratch writes the files of g into a new temporary directory and returns
// the directory. It also writes each file of project at its path, where g
// has no file at that path, such as the build file that a language needs.
// pattern names the directory, as it does for [os.MkdirTemp]. The caller
// removes the directory.
//
// Error modes:
//   - a temporary directory that cannot be created returns the error of
//     [os.MkdirTemp];
//   - a write that fails returns the error of [WriteFile], and Scratch
//     removes the directory.
func Scratch(pattern string, g Generated, project map[string][]byte) (string, error) {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("toolchain: %w", err)
	}
	for path, body := range g.Files {
		if err := WriteFile(dir, path, body); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	for path, body := range project {
		if _, stated := g.Files[path]; stated {
			continue
		}
		if err := WriteFile(dir, path, body); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	return dir, nil
}

// WriteFile writes body to the file at the slash-separated path under dir.
// The file has the mode 0o600, and WriteFile creates each missing directory
// with the mode 0o700.
//
// Error modes:
//   - a path that is not local to dir, such as an absolute path or a path
//     that starts with "..", returns an error and writes nothing;
//   - a directory or a file that cannot be written returns the error of
//     the file system.
func WriteFile(dir, path string, body []byte) error {
	local := filepath.FromSlash(path)
	if !filepath.IsLocal(local) {
		return fmt.Errorf("toolchain: the path %q leaves the directory", path)
	}
	target := filepath.Join(dir, local)
	if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
		return fmt.Errorf("toolchain: %w", err)
	}
	if err := os.WriteFile(target, body, fileMode); err != nil {
		return fmt.Errorf("toolchain: %w", err)
	}
	return nil
}

// Sources returns the paths of the files under dir whose names end in one
// of exts, such as ".go" or ".test.js". It passes over each directory below
// dir whose name is one of skip. The paths are in lexical order, element by
// element, which is the order of [filepath.WalkDir].
//
// Error modes:
//   - a directory that cannot be read returns the error of the walk. For a
//     dir that does not exist, the error wraps [fs.ErrNotExist].
func Sources(dir string, exts []string, skip ...string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && slices.Contains(skip, name) {
				return filepath.SkipDir
			}
			return nil
		}
		if slices.ContainsFunc(exts, func(ext string) bool { return strings.HasSuffix(name, ext) }) {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("toolchain: %w", err)
	}
	return found, nil
}

// Probe writes source to the file at the slash-separated path name under
// dir, calls check, and removes the file. A probe is a file that a harness
// adds to a project to ask its toolchain one question, such as whether a
// type satisfies a contract. check runs the toolchain and records its
// answer, and it returns an error only for a fault of the harness.
//
// Error modes:
//   - a write that fails returns the error of [WriteFile], and Probe does
//     not call check;
//   - otherwise Probe returns the error of check, joined with the error of
//     the removal where the removal fails.
func Probe(dir, name, source string, check func() error) error {
	if err := WriteFile(dir, name, []byte(source)); err != nil {
		return err
	}
	checked := check()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
		return errors.Join(checked, fmt.Errorf("toolchain: %w", err))
	}
	return checked
}
