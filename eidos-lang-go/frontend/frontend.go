// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration: the
// satellite root's constant, restated for the frontend's callers.
const Lang = golang.Lang

// The classification keys are declared at the satellite root beside
// the annotator's: one namespace, one registration, two stamping
// roles.

// UnparsedFile reports a syntax error, positioned at it: the
// source's problem, and the load continues with every declaration
// the parser still recovered.
var UnparsedFile = diag.MustRegister(diag.Prefix("GOLANG"), diag.CodeSpec{
	Number:  1,
	Meaning: "a Go file carries a syntax error",
})

// BadCarrier reports a +-prefixed doc line the kernel grammar
// refused: the carrier attaches nothing and the load continues.
var BadCarrier = diag.MustRegister(diag.Prefix("GOLANG"), diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier on a subject the
// model cannot address, such as a parameter, a result or an import,
// so the author learns that the directive attached nowhere.
var UnaddressedCarrier = diag.MustRegister(diag.Prefix("GOLANG"), diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier sits on a subject the model cannot address",
})

// MixedPackage reports a file inside the build whose package clause
// names another package than the files before it in its directory.
// The load keeps the first name and continues. The go tool refuses
// the directory.
var MixedPackage = diag.MustRegister(diag.Prefix("GOLANG"), diag.CodeSpec{
	Number:  4,
	Meaning: "a directory's Go files declare two package names",
})

// Options is the frontend's declared configuration: one build
// constraint set per load. A tag set is in every unit key by the
// kit's contract, because it changes the graph without changing a
// read; another platform is another load under another
// configuration.
type Options struct {
	// Tags are the build tags this load satisfies. GOOS and GOARCH
	// spellings are tags like any other here.
	Tags []string
}

// Keys registers every golang key: the satellite root's one
// registration, re-exported here where the corpus and the suite
// fixtures reach for it.
var Keys = golang.Keys

// New builds the Go frontend through the kit. A nil options value
// loads with no build tags satisfied.
func New(opts *Options) plugin.Frontend {
	if opts == nil {
		opts = &Options{}
	}
	f := &goFrontend{opts: opts}
	return frontend.New(golang.Name, Lang, golang.Syntax()).
		Version(golang.FrontendVersion).
		Match("**/*"+golang.Extension,
			"!**/testdata/**", "!**/vendor/**", "!**/_*"+golang.Extension, "!**/.*"+golang.Extension,
			"!**/_*/**", "!**/.*/**").
		Units(f.partition).
		Parse(f.parse).
		Classify(markTests).
		Resolve(resolve).
		Options(opts).
		Build()
}

// goFrontend passes the load's configuration to the hooks.
type goFrontend struct {
	opts *Options
}

// markTests stamps the test-file key on every parsed file the go
// tool's own convention names a test.
func markTests(u *plugin.SourceUnit) error {
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		for _, file := range pkg.Files {
			if strings.HasSuffix(file.Path, "_test"+golang.Extension) {
				gb.Stamp(file, meta.RawStamp{
					Key: golang.TestFileKey, Value: true, Pos: file.Pos,
				})
			}
		}
	}
	return nil
}
