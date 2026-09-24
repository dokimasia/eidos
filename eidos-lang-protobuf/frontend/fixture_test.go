// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"os"
	"path/filepath"
	"testing/fstest"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The fixture's one file and the package it declares.
const (
	fixturePath = "svc/store.proto"
	fixturePkg  = "svc.store"
)

// parsed lowers one proto source and returns the unit's builder
// and its sink, so a case reads what loaded and the findings.
func parsed(tb assert.TB, src string, at ...string) (*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	filePath := fixturePath
	if len(at) > 0 {
		filePath = at[0]
	}
	tree := fstest.MapFS{filePath: {Data: []byte(src)}}
	f := protofrontend.New()
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: filePath}}, tree, plugin.DepthFull,
		f.Syntax(), sink, f.Name(),
	)
	assert.NoError(tb, f.Parse(context.Background(), u), "the unit parses")
	return u.Graph(), sink
}

// onlyFile returns the single lowered file.
func onlyFile(tb assert.TB, gb *plugin.GraphBuilder) *node.File {
	tb.Helper()

	assert.Length(tb, gb.Packages(), 1, "one package declared")
	files := gb.Packages()[0].Files
	assert.Length(tb, files, 1, "one file lowered")
	return files[0]
}

// declOf returns the file's declaration of one name, whatever its
// kind, and fails where the file declares none.
func declOf(tb assert.TB, f *node.File, name string) symbol.Symbol {
	tb.Helper()

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *node.Struct:
			if d.Name == name {
				return d
			}
		case *node.Enum:
			if d.Name == name {
				return d
			}
		case *node.Interface:
			if d.Name == name {
				return d
			}
		}
	}
	tb.Fatalf("the file declares no %s", name)
	return nil
}

// stampsOn returns the values stamped on one subject under one
// key, in record order.
func stampsOn(gb *plugin.GraphBuilder, subject symbol.Symbol, key string) []string {
	var out []string
	for _, r := range gb.StampRecords() {
		if r.Subject == subject && string(r.Stamp.Key) == key {
			if text, is := r.Stamp.Value.(string); is {
				out = append(out, text)
			}
		}
	}
	return out
}

// codesOf returns the codes a sink collected.
func codesOf(sink *diag.Sink) []diag.Code {
	out := []diag.Code{}
	for d := range sink.All() {
		out = append(out, d.Code)
	}
	return out
}

// scopeOf returns a resolution scope in one namespace, optionally
// inside one top-level message, for a case reading the candidate
// order directly.
func scopeOf(pkg, owner string) plugin.ImportScope {
	scope := plugin.ImportScope{Bindings: protofrontend.BindingsFor(pkg)}
	if owner != "" {
		scope.Owner = symbol.Identity{Lang: protobuf.Lang, Package: pkg, Name: owner, Kind: symbol.KindStruct}
	}
	return scope
}

// sinkOf returns a fresh diagnostic sink for a case driving the
// frontend directly.
func sinkOf() *diag.Sink { return diag.NewSink() }

// grammar lowers one schema from testdata and returns the unit's
// builder and its sink, so a case reads a schema file on disk.
func grammar(tb assert.TB, name string) (*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	src, err := os.ReadFile(filepath.Join("testdata", "grammar", name))
	assert.NoError(tb, err, name+" is on disk")
	return parsed(tb, string(src), "svc/"+name)
}
