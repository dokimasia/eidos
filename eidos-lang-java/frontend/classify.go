// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The names that mark a test file: the directory Maven's and Gradle's
// layout keeps test sources in, and the prefix and suffixes Surefire's
// default includes match a test class's name by.
const (
	testSourceDir  = "src/test/"
	testPrefix     = "Test"
	testSuffix     = "Test"
	testsSuffix    = "Tests"
	testCaseSuffix = "TestCase"
)

// markTests stamps java.testFile on every File node of a test file.
func markTests(u *plugin.SourceUnit) error {
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		for _, file := range pkg.Files {
			if testFile(file.Path) {
				gb.Stamp(file, meta.RawStamp{Key: java.TestFileKey, Value: true, Pos: file.Pos})
			}
		}
	}
	return nil
}

// testFile reports whether a file is a test file: it is under a src/test/
// directory, or Surefire's default includes name it, Test*.java,
// *Test.java, *Tests.java and *TestCase.java.
func testFile(p string) bool {
	if strings.HasPrefix(p, testSourceDir) || strings.Contains(p, "/"+testSourceDir) {
		return true
	}
	stem := strings.TrimSuffix(path.Base(p), java.Extension)
	return strings.HasPrefix(stem, testPrefix) || strings.HasSuffix(stem, testSuffix) ||
		strings.HasSuffix(stem, testsSuffix) || strings.HasSuffix(stem, testCaseSuffix)
}
