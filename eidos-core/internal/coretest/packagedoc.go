// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest

import (
	"go/token"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// dependencyHeading is the section a package comment states its
// dependency position under.
const dependencyHeading = "# Dependency position"

// AssertDependencyPosition fails unless the package under test
// carries a package comment stating its dependency position.
//
// The documentation is part of the contract: every other package
// relies on where this one sits, and a position stated in a review
// comment rots. It reads the package's own source in the working
// directory, which is the directory of the package under test.
func AssertDependencyPosition(tb testing.TB) {
	tb.Helper()

	states, err := StatesDependencyPosition(".")
	if err != nil {
		tb.Fatalf("StatesDependencyPosition: %v", err)
	}
	if !states {
		tb.Fatal("no file carries the package comment with its dependency position")
	}
}

// StatesDependencyPosition reports whether the package in dir
// carries a package comment stating its dependency position.
//
// Only hand-written files are read: a generated file carries the
// generator's preamble rather than the package's own contract.
func StatesDependencyPosition(dir string) (bool, error) {
	files, err := gosource.ParseDir(token.NewFileSet(), dir, gosource.HandWritten)
	if err != nil {
		return false, err
	}
	for _, file := range files {
		if file.Doc == nil {
			continue
		}
		if strings.Contains(file.Doc.Text(), dependencyHeading) {
			return true, nil
		}
	}
	return false, nil
}
