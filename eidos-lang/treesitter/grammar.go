// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter

import (
	"context"
	"fmt"
	"path"
	"runtime/debug"
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// runtimeModule is the module path of the tree-sitter runtime binding,
// whose version [Grammar.Version] names beside the grammar's.
const runtimeModule = "github.com/tree-sitter/go-tree-sitter"

// errorKind is the id the runtime gives every ERROR node, the one kind
// no grammar's table lists, and errorName is its name.
const (
	errorKind Kind = 65535
	errorName      = "ERROR"
)

// Kind is the id of one node kind of a grammar, as the grammar's
// parser assigns it. An id means nothing outside its [Grammar], and
// the zero Kind is the kind of no node.
type Kind uint16

// Field is the id of one field name of a grammar. The zero Field
// names no field.
type Field uint16

// Grammar is one pinned tree-sitter language: its node kinds and field
// names, resolved once when it loads, and the module versions a
// frontend folds into its own version. A Grammar is immutable and safe
// for concurrent use.
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
func Load(name, module string, language unsafe.Pointer) *Grammar {
	lang := ts.NewLanguage(language)
	abi := lang.AbiVersion()
	if abi < ts.MIN_COMPATIBLE_LANGUAGE_VERSION || abi > ts.LANGUAGE_VERSION {
		panic(fmt.Sprintf("treesitter: grammar %s has ABI version %d, and the runtime accepts %d to %d",
			name, abi, ts.MIN_COMPATIBLE_LANGUAGE_VERSION, ts.LANGUAGE_VERSION))
	}
	g := &Grammar{
		name:     name,
		version:  moduleVersion(module) + ", " + moduleVersion(runtimeModule),
		language: lang,
		kinds:    make([]string, lang.NodeKindCount()),
		named:    map[string]Kind{errorName: errorKind},
		fields:   map[string]Field{},
	}
	for id := range g.kinds {
		name := lang.NodeKindForId(uint16(id))
		g.kinds[id] = name
		if _, met := g.named[name]; met {
			continue
		}
		// Several ids can spell one name, as an alias does, and every
		// node of the name reports the one the runtime's own lookup
		// returns.
		if public := publicKind(lang, name); public != 0 {
			g.named[name] = public
		}
	}
	for id := range lang.FieldCount() {
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
func (g *Grammar) Parse(ctx context.Context, file string, src []byte) (*Tree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p := ts.NewParser()
	defer p.Close()
	// Load checked the ABI version, which is the one refusal.
	_ = p.SetLanguage(g.language)
	tree := p.Parse(src, nil)
	return &Tree{grammar: g, path: file, src: src, tree: tree, root: rootOf(tree)}, nil
}

// develVersion is the version of a module a replace directive points
// at a directory, which has none. Edits in that directory re-key
// nothing.
const develVersion = "(devel)"

// moduleVersion returns a module's base name and the version the
// running binary was built with, the replacement's where a replace
// directive names one. It panics on a binary whose build information
// does not list the module.
func moduleVersion(module string) string {
	info, built := debug.ReadBuildInfo()
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
		version := dep.Version
		if version == "" {
			version = develVersion
		}
		return path.Base(module) + " " + version
	}
	panic(fmt.Sprintf("treesitter: the binary's build information does not list %s", module))
}
