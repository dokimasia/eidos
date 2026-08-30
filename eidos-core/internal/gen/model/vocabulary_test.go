// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package model_test

import (
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// generatorPackages are the packages a generation run compiles in.
// Each is checked against the dependency rule below.
var generatorPackages = []string{
	"internal/gen/model",
	"internal/gen/model/cmd",
	"internal/genfile",
	"internal/gosource",
}

func TestVocabulary(t *testing.T) {
	t.Parallel()

	// The kernel builds its models without a language satellite, and
	// a satellite depends on the kernel. A generator that imported
	// the model it generates could never bootstrap: the first run
	// would need output that does not exist yet.
	t.Run("dependency position", func(t *testing.T) {
		t.Parallel()

		root, err := gosource.ModuleRoot(".")
		if err != nil {
			t.Fatalf("ModuleRoot: %v", err)
		}
		modPath, err := gosource.ModulePath(root)
		if err != nil {
			t.Fatalf("ModulePath: %v", err)
		}

		for _, pkg := range generatorPackages {
			t.Run(pkg, func(t *testing.T) {
				t.Parallel()

				files, err := gosource.ParseDir(
					token.NewFileSet(), filepath.Join(root, pkg), gosource.HandWritten,
				)
				if err != nil {
					t.Fatalf("ParseDir: %v", err)
				}
				for _, file := range files {
					for _, imported := range file.Imports {
						path, err := strconv.Unquote(imported.Path.Value)
						if err != nil {
							t.Fatalf("unquote %s: %v", imported.Path.Value, err)
						}
						assertAllowed(t, modPath, path)
					}
				}
			})
		}
	})
}

// assertAllowed fails unless an import is a standard-library
// package or one of the generator's own internal helpers.
func assertAllowed(t *testing.T, modPath, path string) {
	t.Helper()

	local, isLocal := strings.CutPrefix(path, modPath+"/")
	if !isLocal {
		// A path whose first segment carries a dot names a module,
		// so anything else is the standard library.
		if first, _, _ := strings.Cut(path, "/"); strings.Contains(first, ".") {
			t.Fatalf("imports %s: the generator takes no third-party dependency", path)
		}
		return
	}
	if !strings.HasPrefix(local, "internal/") {
		t.Fatalf("imports %s: the generator imports no kernel package outside internal/, "+
			"because a generator that imported the model it generates could never bootstrap",
			path)
	}
}
