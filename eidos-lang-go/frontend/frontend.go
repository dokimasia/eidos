// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	sdk "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language every loaded declaration carries.
const Lang symbol.Lang = "golang"

// TestFileKey classifies a file the go tool would treat as a test.
const TestFileKey meta.KeyName = "golang.testFile"

// ConstraintKey classifies a file whose build constraint falls
// outside the load's tag set; the value is the constraint line as
// written.
const ConstraintKey meta.KeyName = "golang.constraint"

// UnparsedFile reports a Go file the parser refused: the source's
// problem, positioned, and the load continues around it.
var UnparsedFile = diag.MustRegister(diag.Prefix("GOLANG"), diag.CodeSpec{
	Number:  1,
	Meaning: "a Go file failed to parse",
})

// BadCarrier reports a +-prefixed doc line the kernel grammar
// refused: the carrier attaches nothing and the load continues.
var BadCarrier = diag.MustRegister(diag.Prefix("GOLANG"), diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
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

// Keys registers the classification keys this frontend stamps, in
// the shape a composition and a corpus fixture declare them.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace("golang", string(golang.Name)); err != nil {
		return err
	}
	if _, err := meta.Register[bool](r, meta.KeySpec{
		Name:  TestFileKey,
		Kinds: []symbol.Kind{symbol.KindFile},
		Doc:   "marks a file the go tool treats as a test",
	}); err != nil {
		return err
	}
	_, err := meta.Register[string](r, meta.KeySpec{
		Name:  ConstraintKey,
		Kinds: []symbol.Kind{symbol.KindFile},
		Doc:   "carries the build constraint that kept a file's declarations out",
	})
	return err
}

// New builds the Go frontend through the kit. A nil options value
// loads with no build tags satisfied.
func New(opts *Options) plugin.Frontend {
	if opts == nil {
		opts = &Options{}
	}
	f := &goFrontend{opts: opts}
	return sdk.NewFrontend(golang.Name, Lang, golang.Syntax()).
		Version(golang.Version).
		Match("**/*"+golang.Extension, "!**/testdata/**").
		Units(f.partition).
		Parse(f.parse).
		Classify(markTests).
		Resolve(resolve).
		Options(opts).
		Build()
}

// goFrontend carries the load's configuration into the hooks.
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
					Key: TestFileKey, Value: true, Pos: file.Pos,
				})
			}
		}
	}
	return nil
}
