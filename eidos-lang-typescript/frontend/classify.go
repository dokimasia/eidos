// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"slices"
	"strings"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The names Jest's default match reads a test file by: the directory a
// test file is under, a file named test or spec, and a name that ends
// in .test or .spec before its extension.
const (
	testsDir   = "__tests__"
	testName   = "test"
	specName   = "spec"
	testSuffix = ".test"
	specSuffix = ".spec"
)

// markTests stamps the test-file key on every File node of a file
// Jest's default match names a test.
func markTests(u *plugin.SourceUnit) error {
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		for _, file := range pkg.Files {
			if testFile(file.Path) {
				gb.Stamp(file, meta.RawStamp{Key: typescript.TestFileKey, Value: true, Pos: file.Pos})
			}
		}
	}
	return nil
}

// testFile reports whether Jest's default match names a file a test:
// it is under a __tests__ directory, or its name before its extension
// is test or spec or ends in .test or .spec.
func testFile(p string) bool {
	if slices.Contains(strings.Split(path.Dir(p), "/"), testsDir) {
		return true
	}
	base := stripExtension(path.Base(p))
	return base == testName || base == specName ||
		strings.HasSuffix(base, testSuffix) || strings.HasSuffix(base, specSuffix)
}
