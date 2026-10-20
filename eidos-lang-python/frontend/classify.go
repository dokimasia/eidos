// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
)

// The names by which the default collection of pytest finds a test file:
// the prefix and the suffix of the stem of a test module, and the base name
// of the module of the fixtures of a directory.
const (
	testPrefix = "test_"
	testSuffix = "_test"
	conftest   = "conftest" + python.Extension
)

// testFile reports whether the default collection of pytest finds a file as
// a test: a module whose stem starts with test_ or ends with _test, which
// is the default python_files of pytest, and every conftest.py. A stub file
// is no test. The kit stamps python.testFile on every file for which it
// reports true.
func testFile(p string) bool {
	base := path.Base(p)
	stem, module := strings.CutSuffix(base, python.Extension)
	return module && (base == conftest || strings.HasPrefix(stem, testPrefix) || strings.HasSuffix(stem, testSuffix))
}
