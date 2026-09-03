// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"fmt"
	"strings"

	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The fixture module and the files a healthy project holds.
const (
	fixtureModule = "eidos.test/harness"
	rowFile       = "row.go"
	rowTestFile   = "row_test.go"
)

// healthy returns generated output that parses, type-checks, vets
// and whose one test passes.
func healthy() toolchain.Generated {
	return toolchain.Generated{
		Module: fixtureModule,
		Files: map[string][]byte{
			rowFile: []byte(`package harness

// Row is a generated record.
type Row struct {
	ID   int
	Name string
}

// Reader is what a Row satisfies.
type Reader interface {
	Read() string
}

// Read returns the row's name.
func (r *Row) Read() string { return r.Name }
`),
			rowTestFile: []byte(`package harness

import "testing"

func TestRow(t *testing.T) {
	r := &Row{ID: 1, Name: "a"}
	if got := r.Read(); got != "a" {
		t.Fatalf("Read() = %q, want a", got)
	}
}
`),
		},
	}
}

// with returns the healthy fixture carrying one file replaced or
// added, for a case that breaks exactly one thing.
func with(path, body string) toolchain.Generated {
	g := healthy()
	g.Files[path] = []byte(body)
	return g
}

// only returns a fixture holding one file, for a case that needs no
// test file.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{
		Module: fixtureModule,
		Files:  map[string][]byte{path: []byte(body)},
	}
}

// recorder is a test handle collecting what an assertion reported,
// so a case reads the outcome rather than failing the run.
type recorder struct {
	failures []string
	skipped  string
}

func (*recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Skip(args ...any) { r.skipped = fmt.Sprint(args...) }

func (r *recorder) failed() bool { return len(r.failures) > 0 }

func (r *recorder) says(text string) bool {
	for _, f := range r.failures {
		if strings.Contains(f, text) {
			return true
		}
	}
	return false
}

// adapter is the harness under test.
func adapter() toolchain.Adapter { return gotesting.New() }
