// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter

import (
	"context"
	"fmt"
	"path"
	"runtime/debug"
	"sync"
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// runtimeModule is the module path of the tree-sitter runtime binding,
// whose version [Grammar.Version] names beside the grammar's.
const runtimeModule = "github.com/tree-sitter/go-tree-sitter"

// develVersion is the version of a module a replace directive points
// at a directory, which has none. Edits in that directory re-key
// nothing.
const develVersion = "(devel)"

// buildInfo reads the running binary's build information once, for the
// versions of every grammar the process loads.
var buildInfo = sync.OnceValues(debug.ReadBuildInfo)

// Kind is the id of one node kind of a grammar, as the grammar's
// parser assigns it. An id means nothing outside its [Grammar], and
// the zero Kind is the kind of no node.
type Kind uint16

// errorKind is the id the runtime gives every ERROR node, the one kind
// no grammar's table lists, and errorName is its name.
const (
	errorKind Kind = 65535
	errorName      = "ERROR"
)

// Field is the id of one field name of a grammar. The zero Field
// names no field.
type Field uint16

// Grammar is one pinned tree-sitter language: its node kinds and field
// names, resolved once when it loads, and the module versions a
// frontend folds into its own version.
//
// # Concurrency
//
// A Grammar is immutable after [Load] and safe for concurrent use, so
// parses of many files run in parallel.
//
// # Allocation contract
//
// [Grammar.Name], [Grammar.Version], [Grammar.Kind],
// [Grammar.KindName] and [Grammar.Field] allocate nothing.
// [Grammar.Parse] allocates the [Tree] it returns.
type Grammar struct {
	name     string
	version  string
	language *ts.Language
	// kinds names every kind by its id. named maps the name of each
	// named kind onto its id, and fields the name of each field.
	kinds  []string
	named  map[string]Kind
	fields map[string]Field
}

// Load wraps the language pointer a grammar binding returns, under the
// grammar's name and the path of the module the binding is in. It
// reads the versions of that module and of the runtime binding from the
// running binary's build information. The grammar packages beneath
// this one call it once each, when they initialize.
//
// Load panics on a language whose ABI version the runtime does not
// accept, and on a binary whose build information does not list either
// module, because a unit key that folds no grammar version would serve
// a stale graph across an upgrade.
//
// # Allocation contract
//
// Load allocates the grammar and its language handle, the version, the
// list of kinds and the two maps, each sized to the grammar's tables,
// and one copy of each kind name and field name that the binding reads
// from C. It reads the build information once per process.
func Load(name, module string, language unsafe.Pointer) *Grammar {
	lang := ts.NewLanguage(language)
	abi := lang.AbiVersion()
	if abi < ts.MIN_COMPATIBLE_LANGUAGE_VERSION || abi > ts.LANGUAGE_VERSION {
		panic(fmt.Sprintf("treesitter: grammar %s has ABI version %d, and the runtime accepts %d to %d",
			name, abi, ts.MIN_COMPATIBLE_LANGUAGE_VERSION, ts.LANGUAGE_VERSION))
	}
	grammarModule, grammarVersion := moduleVersion(module)
	runtimeName, runtimeVersion := moduleVersion(runtimeModule)
	kinds, fields := lang.NodeKindCount(), lang.FieldCount()
	g := &Grammar{
		name:     name,
		version:  grammarModule + " " + grammarVersion + ", " + runtimeName + " " + runtimeVersion,
		language: lang,
		kinds:    make([]string, kinds),
		named:    make(map[string]Kind, kinds+1),
		fields:   make(map[string]Field, fields),
	}
	g.named[errorName] = errorKind
	for id := range g.kinds {
		kind := lang.NodeKindForId(uint16(id))
		g.kinds[id] = kind
		if _, met := g.named[kind]; met {
			continue
		}
		// Several ids can spell one name, as an alias does, and every
		// node of the name reports the one the runtime's own lookup
		// returns.
		if public := publicKind(lang, kind); public != 0 {
			g.named[kind] = public
		}
	}
	for id := range fields {
		field := uint16(id + 1)
		g.fields[lang.FieldNameForId(field)] = Field(field)
	}
	return g
}

// Name returns the grammar's name, such as "typescript".
func (g *Grammar) Name() string { return g.name }

// Version returns the grammar's module and the runtime binding's module
// at the versions the running binary was built with, as in
// "tree-sitter-java v0.23.5, go-tree-sitter v0.25.0".
func (g *Grammar) Version() string { return g.version }

// Kind returns the id of a named node kind, and of the ERROR kind for
// "ERROR". It panics on a name the grammar does not declare, so a
// frontend written against another grammar version fails when it is
// constructed and not during a parse.
func (g *Grammar) Kind(name string) Kind {
	k, declared := g.named[name]
	if !declared {
		panic(fmt.Sprintf("treesitter: grammar %s declares no named node kind %q", g.name, name))
	}
	return k
}

// KindName returns the name of a kind, and nothing for an id the
// grammar does not assign.
func (g *Grammar) KindName(k Kind) string {
	if k == errorKind {
		return errorName
	}
	if int(k) < len(g.kinds) {
		return g.kinds[k]
	}
	return ""
}

// Field returns the id of a field name, and panics as [Grammar.Kind]
// does on a name the grammar does not declare.
func (g *Grammar) Field(name string) Field {
	f, declared := g.fields[name]
	if !declared {
		panic(fmt.Sprintf("treesitter: grammar %s declares no field %q", g.name, name))
	}
	return f
}

// Parse parses one file, whose slash path every position of the tree
// names. It returns ctx.Err() without parsing when the context is done.
// Otherwise it returns a tree for any input: the parse sets no
// timeout, cancellation flag or progress callback, so the runtime
// returns a tree for every input, and a syntax error is an ERROR or
// MISSING node inside it. The caller closes the tree. The tree keeps
// src, which the caller does not change while the tree is open.
//
// The runtime reads src in place, through a parser that Parse creates
// and deletes before it returns.
//
// # Allocation contract
//
// Parse allocates the [Tree] it returns, one allocation on the Go
// heap. The tree's nodes are C memory until [Tree.Close].
func (g *Grammar) Parse(ctx context.Context, file string, src []byte) (*Tree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tree := parseString(g.language, src)
	return &Tree{grammar: g, path: file, src: src, tree: tree, root: rootNode(tree)}, nil
}

// moduleVersion returns a module's base name and the version the
// running binary was built with, the replacement's where a replace
// directive names one. It panics on a binary whose build information
// does not list the module.
func moduleVersion(module string) (name, version string) {
	info, built := buildInfo()
	if !built {
		panic("treesitter: the binary records no build information to read a grammar's version from")
	}
	for _, dep := range info.Deps {
		if dep.Path != module {
			continue
		}
		if dep.Replace != nil {
			dep = dep.Replace
		}
		version = dep.Version
		if version == "" {
			version = develVersion
		}
		return path.Base(module), version
	}
	panic(fmt.Sprintf("treesitter: the binary's build information does not list %s", module))
}
