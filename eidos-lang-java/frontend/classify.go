// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/java"
)

// The names that mark a test file: the directory of the test sources in
// the layout of Maven and Gradle, and the prefix and the suffixes by which
// the default includes of Surefire match the name of a test class.
const (
	testSourceDir  = "src/test/"
	testPrefix     = "Test"
	testSuffix     = "Test"
	testsSuffix    = "Tests"
	testCaseSuffix = "TestCase"
)

// testFile reports whether a file is a test file: a file under a src/test/
// directory, or a file that the default includes of Surefire match,
// Test*.java, *Test.java, *Tests.java and *TestCase.java.
func testFile(p string) bool {
	if strings.HasPrefix(p, testSourceDir) || strings.Contains(p, "/"+testSourceDir) {
		return true
	}
	stem := strings.TrimSuffix(path.Base(p), java.Extension)
	return strings.HasPrefix(stem, testPrefix) || strings.HasSuffix(stem, testSuffix) ||
		strings.HasSuffix(stem, testsSuffix) || strings.HasSuffix(stem, testCaseSuffix)
}
