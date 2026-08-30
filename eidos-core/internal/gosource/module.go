// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package gosource

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// goModName is the file that marks a module root.
const goModName = "go.mod"

// moduleDirective opens the go.mod line naming the module.
const moduleDirective = "module"

// ModuleRoot walks up from dir to the directory holding go.mod.
//
// It answers an absolute path, so a caller may join relative
// package directories onto it. Reaching the filesystem root without
// finding go.mod is an error naming the directory the walk started
// from.
func ModuleRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("gosource: resolve %s: %w", dir, err)
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, goModName)); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("gosource: no %s above %s", goModName, dir)
		}
		abs = parent
	}
}

// ModulePath reads the module path from a module root's go.mod.
//
// It reads the directive textually rather than through the module
// tooling, because resolving a module graph would make generation
// depend on the network and on a populated module cache. A quoted
// path is unquoted; anything else is taken verbatim.
func ModulePath(modRoot string) (string, error) {
	body, err := os.ReadFile(filepath.Join(modRoot, goModName))
	if err != nil {
		return "", fmt.Errorf("gosource: read %s: %w", goModName, err)
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != moduleDirective {
			continue
		}
		if unquoted, err := strconv.Unquote(fields[1]); err == nil {
			return unquoted, nil
		}
		return fields[1], nil
	}
	return "", fmt.Errorf("gosource: no %s directive in %s", moduleDirective,
		filepath.Join(modRoot, goModName))
}
