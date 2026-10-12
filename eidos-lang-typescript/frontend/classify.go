// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
)

// The names by which the default match of Jest finds a test file: the
// directory that a test file is under, a file named test or spec, and a
// name that ends in .test or .spec before its extension.
const (
	testsDir   = "__tests__"
	testName   = "test"
	specName   = "spec"
	testSuffix = ".test"
	specSuffix = ".spec"
)

// testFile reports whether the default match of Jest finds a file as a
// test. The file is under a __tests__ directory, or its name before its
// extension is test or spec or ends in .test or .spec. The kit stamps
// typescript.testFile on every file for which it reports true.
func testFile(p string) bool {
	if slices.Contains(strings.Split(path.Dir(p), "/"), testsDir) {
		return true
	}
	base := typescript.ModulePath(path.Base(p))
	return base == testName || base == specName ||
		strings.HasSuffix(base, testSuffix) || strings.HasSuffix(base, specSuffix)
}
