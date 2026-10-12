// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration. It restates the
// constant of the satellite root for the callers of the frontend.
const Lang = golang.Lang

// UnparsedFile reports a syntax error at its position. The error is a
// problem of the source, and the load continues with every declaration
// that the parser recovered.
var UnparsedFile = diag.MustRegister(golang.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a Go file has a syntax error",
})

// BadCarrier reports a doc line with the + prefix that the kernel grammar
// refused. The carrier attaches nothing, and the load continues.
var BadCarrier = diag.MustRegister(golang.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier on a subject that the
// model cannot address, such as a parameter, a result or an import. The
// author learns that the directive attached nowhere.
var UnaddressedCarrier = diag.MustRegister(golang.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// MixedPackage reports a file inside the build whose package clause names
// another package than the files before it in its directory. The load
// keeps the first name and continues. The go tool refuses the directory.
var MixedPackage = diag.MustRegister(golang.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a directory's Go files declare two package names",
})

// Options is the declared configuration of the frontend: one set of build
// constraints per load. The kit folds a tag set into every unit key,
// because the set changes the graph without changing a read. Another
// platform is another load under another configuration.
type Options struct {
	// Tags are the build tags that this load satisfies. The spellings of
	// GOOS and GOARCH are tags like any other here.
	Tags []string
}

// New builds the Go frontend through the kit. Nil options load with no
// build tag satisfied. The frontend registers every golang key through its
// role, under the identity of the language. The classification keys are
// declared at the satellite root beside the keys of the annotator, so one
// namespace has one registration and two roles stamp it. Its classifier
// stamps golang.testFile on every file whose name ends in _test.go, the
// convention of the go command.
//
// # Allocation contract
//
// New allocates the state of the frontend and its three hooks, the
// classifier of test files, two allocations of the syntax and five of the
// kit: 12 allocations, and 13 with the empty options that nil options
// take.
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
		Classify(frontend.TestFiles(golang.TestFileKey, func(p string) bool {
			return strings.HasSuffix(p, golang.TestSuffix+golang.Extension)
		})).
		Resolve(resolve).
		Options(opts).
		Dependencies(f.dependencies).
		Stores(Stores).
		Keys(golang.Keys).
		Build()
}

// goFrontend passes the configuration of the load to the hooks.
type goFrontend struct {
	opts *Options
}
