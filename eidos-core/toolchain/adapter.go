// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"io/fs"

	"go.dokimi.dev/eidos/core/symbol"
)

// Generated is what a run produced, as an assertion receives it:
// the rendered files and whatever a compiler needs beside them to
// read them.
//
// A fixture is a value. The adapter lays it out on disk; nothing
// here writes a file, because where a scratch project sits and
// what it must contain is the language's own question.
type Generated struct {
	// Files are the rendered outputs, keyed by the path each took
	// in the output tree, slash-separated.
	Files map[string][]byte
	// Sources is the tree the run read, for a language whose
	// generated output refers back to it. A fixture whose output
	// stands alone leaves it nil.
	Sources fs.FS
	// Module is the module or namespace identity the laid-out
	// project declares, for a language that needs one to resolve
	// its own imports.
	Module string
}

// IsEmpty reports whether the fixture carries no output, which
// every assertion refuses: a toolchain run over nothing passes
// while proving nothing.
func (g Generated) IsEmpty() bool { return len(g.Files) == 0 }

// TestReport is what running a language's tests returned.
type TestReport struct {
	// Passed, Failed and Skipped count the cases the run reported.
	Passed, Failed, Skipped int
	// Output is the run's own text, for a failure message that
	// names what the toolchain said rather than only that it
	// spoke.
	Output string
}

// OK reports whether every case that ran passed and at least one
// did: a report of nothing is not a pass, for the reason an empty
// fixture is not one.
func (r TestReport) OK() bool { return r.Failed == 0 && r.Passed > 0 }

// Adapter is what one language states about its own toolchain.
//
// Every method takes or returns a plain value, so the kernel drives
// a compiler it knows nothing about. An operation the language's
// tooling cannot perform returns an error saying so, which the
// assertion reports as the failure it is; nothing here guesses at a
// missing capability.
type Adapter interface {
	// Lang names the language, for the assertions' wording.
	Lang() symbol.Lang
	// Available reports whether the toolchain is on this machine,
	// and the reason it is not where it is absent. The reason is
	// what a local skip records and what a CI failure names.
	Available() (bool, string)
	// Layout writes a scratch project the toolchain accepts and
	// returns its directory. The caller removes it.
	Layout(g Generated) (dir string, err error)
	// Parse reads the project's syntax and nothing else, so a
	// syntax error is told apart from a type error.
	Parse(dir string) error
	// TypeCheck holds the project to its language's type rules.
	TypeCheck(dir string) error
	// RunTests runs the project's own tests.
	RunTests(dir string) (TestReport, error)
	// Satisfies reports whether one type meets one contract: an
	// interface, a trait, a protocol, whatever the language calls
	// the shape a value is checked against.
	Satisfies(dir, typeName, contract string) (bool, error)
}
