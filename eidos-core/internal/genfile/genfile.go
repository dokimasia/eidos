// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package genfile

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Set is a generator's whole output: module-relative slash paths
// against the bytes each file holds.
//
// Paths are slash-separated on every platform, so a set compares
// equal wherever it was produced.
type Set map[string][]byte

// The suffixes marking a generated Go file. The mirror guard reads
// them to tell its own output from hand-written source, and a
// generator that emits tests alongside its code is covered by both.
const (
	// GeneratedSuffix marks generated source.
	GeneratedSuffix = ".gen.go"
	// GeneratedTestSuffix marks a generated test, which does not end
	// in GeneratedSuffix and would otherwise escape the guard.
	GeneratedTestSuffix = ".gen_test.go"
)

// dirPerm and filePerm are the modes generated output lands with.
const (
	dirPerm  fs.FileMode = 0o750
	filePerm fs.FileMode = 0o600
)

// Format canonicalizes one generated file through gofmt.
//
// path names the file in the error only. A file that does not parse
// is a defect in the template that rendered it, so the error names
// the file and wraps the parse failure rather than writing
// unformatted bytes.
func Format(path string, src []byte) ([]byte, error) {
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("genfile: format %s: %w", path, err)
	}
	return out, nil
}

// Write puts every file of the set on disk under root, creating
// directories as needed.
//
// A path that escapes root is refused before anything is written,
// so a template that composes a bad path cannot scatter files
// outside the module.
func Write(root string, set Set) error {
	for _, path := range slices.Sorted(maps(set)) {
		target, err := resolve(root, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
			return fmt.Errorf("genfile: create directory for %s: %w", path, err)
		}
		if err := os.WriteFile(target, set[path], filePerm); err != nil {
			return fmt.Errorf("genfile: write %s: %w", path, err)
		}
	}
	return nil
}

// Verify reports whether the tree under root matches the set: the
// mirror guard.
//
// dirs are the module-relative directories the generator owns, and
// every generated file found in them has to belong to the set. The
// error names every problem it found, in path order, so one run
// says everything that is wrong.
func Verify(root string, set Set, dirs []string) error {
	var problems []string

	for _, path := range slices.Sorted(maps(set)) {
		target, err := resolve(root, path)
		if err != nil {
			return err
		}
		onDisk, err := os.ReadFile(target)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			problems = append(problems, fmt.Sprintf("%s is missing", path))
		case err != nil:
			return fmt.Errorf("genfile: read %s: %w", path, err)
		case !bytes.Equal(onDisk, set[path]):
			problems = append(problems, fmt.Sprintf("%s differs from what the generator produces", path))
		}
	}

	strays, err := strays(root, set, dirs)
	if err != nil {
		return err
	}
	for _, path := range strays {
		problems = append(problems, fmt.Sprintf("%s is generated but no generator claims it", path))
	}

	if len(problems) == 0 {
		return nil
	}
	slices.Sort(problems)
	return fmt.Errorf("genfile: the tree does not match the generator:\n  %s",
		strings.Join(problems, "\n  "))
}

// strays lists generated files under dirs that the set does not
// name, in path order.
func strays(root string, set Set, dirs []string) ([]string, error) {
	var found []string
	for _, dir := range dirs {
		base, err := resolve(root, dir)
		if err != nil {
			return nil, err
		}
		walkErr := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			switch {
			case errors.Is(err, fs.ErrNotExist):
				return nil
			case err != nil:
				return err
			case d.IsDir() || !generated(d.Name()):
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if slash := filepath.ToSlash(rel); set[slash] == nil {
				found = append(found, slash)
			}
			return nil
		})
		if walkErr != nil && !errors.Is(walkErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("genfile: scan %s: %w", dir, walkErr)
		}
	}
	slices.Sort(found)
	return found, nil
}

// generated reports whether a filename is generated output.
func generated(name string) bool {
	return strings.HasSuffix(name, GeneratedSuffix) ||
		strings.HasSuffix(name, GeneratedTestSuffix)
}

// resolve joins a module-relative path onto root, refusing one that
// would escape it.
func resolve(root, path string) (string, error) {
	local := filepath.FromSlash(path)
	if !filepath.IsLocal(local) {
		return "", fmt.Errorf("genfile: %s escapes the module root", path)
	}
	return filepath.Join(root, local), nil
}

// maps yields a set's paths, so callers can sort them.
func maps(set Set) func(func(string) bool) {
	return func(yield func(string) bool) {
		for path := range set {
			if !yield(path) {
				return
			}
		}
	}
}
