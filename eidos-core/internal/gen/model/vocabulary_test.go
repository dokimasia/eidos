// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

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
		assert.NoError(t, err, "the module root resolves")
		modPath, err := gosource.ModulePath(root)
		assert.NoError(t, err, "the module path reads")

		for _, pkg := range generatorPackages {
			t.Run(pkg, func(t *testing.T) {
				t.Parallel()

				files, err := gosource.ParseDir(
					token.NewFileSet(), filepath.Join(root, pkg), gosource.HandWritten,
				)
				assert.NoError(t, err, "the generator package parses")
				for _, file := range files {
					for _, imported := range file.Imports {
						path, err := strconv.Unquote(imported.Path.Value)
						assert.NoError(t, err, "the import path unquotes")
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
		first, _, _ := strings.Cut(path, "/")
		assert.False(t, strings.Contains(first, "."),
			"the generator takes no third-party dependency")
		return
	}
	assert.HasPrefix(t, local, "internal/",
		"the generator imports no kernel package outside internal/, because a "+
			"generator that imported the model it generates could never bootstrap")
}
