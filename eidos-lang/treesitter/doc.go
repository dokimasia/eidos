// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package treesitter parses source with a pinned tree-sitter grammar
// and returns the syntax tree through its own types, so no type of the
// tree-sitter binding appears in a satellite.
//
// A [Grammar] is one pinned language. The grammar packages beneath
// this one each load theirs once, when they initialize, and a frontend
// resolves the [Kind] and [Field] ids it walks by at construction:
// [Grammar.Kind] and [Grammar.Field] panic on a name the grammar does
// not declare, so a frontend written against another grammar version
// fails when it is built and not during a parse. [Grammar.Parse]
// returns a [Tree] for any input, because a syntax error is an ERROR
// or MISSING node inside the tree and never an error return, and
// [Tree.Errors] yields those nodes. A [Node] is a value handle on one
// syntax node.
//
// # Spellings
//
// [Node.Text] returns a node's source bytes as written. [Node.Compact]
// returns its tokens without the whitespace between them and without
// its comments, one space kept between two identifier tokens, so a
// frontend that spells a type through it spells one signature one way
// however the source is formatted. [Node.TextThrough] returns the
// source from one node through another as written, the text of a run
// of sibling tokens no single node spans, such as one argument of a
// token tree. [Node.AllChildren] yields the grammar's keywords and
// punctuation beside the named children, which is where a grammar
// states a modifier such as static or readonly, and [Node.IsExtra]
// reports the extras among them, such as comments.
//
// # Versions
//
// [Grammar.Version] names the grammar's module and the runtime
// binding's module at the versions the running binary was built with.
// A frontend appends it to the version every unit key folds, so an
// upgrade of either module re-keys every unit the grammar parsed.
//
// # Memory
//
// A tree's nodes are C memory the runtime allocates. [Tree.Close]
// releases them, and a second Close releases nothing. Every node of a
// closed tree reports [Node.IsZero] and returns zero values. The
// package calls no binding function that leaks at the pinned runtime
// version:
//
//   - It resolves each kind once, through the runtime's own lookup,
//     which its cgo preamble declares, and passes the name's bytes. The
//     binding's wrapper copies the name into C memory it never frees.
//   - It reads the field table once, and a child by the field's id.
//   - It parses through the runtime's own string parse, which reads the
//     source in place. The binding's parse copies the source into Go
//     and C memory for each read through a call back into Go.
//
// Each parse creates the runtime's parser and deletes it before it
// returns.
//
// # Cost
//
// The package parses, reads a node and walks a tree through the
// runtime's own C functions, which its cgo preamble declares. It keeps
// each node and each cursor by value in Go memory. A walk allocates
// nothing on the Go heap, and a parse allocates its [Tree] alone. The
// binding returns every node it reads as a new heap allocation.
//
// A walk of a node's children makes one call into C per child it
// yields, whether it walks all children, the named ones or the ones
// under one field. [Node.Compact] makes one call per node.
//
// When it initializes, the package sets the runtime's allocator back to
// libc's. The binding routes every allocation of the runtime through a
// call back into Go, and allocates from libc too. The package's calls
// into C then never call back into Go. A test run would report such a
// call as a panic.
//
// # Concurrency
//
// A [Grammar] is immutable and safe for concurrent use. Parses of many
// files run in parallel. One goroutine uses a [Tree] and its nodes at a
// time.
//
// # Positions
//
// [Node.Pos] and [Node.End] add one to the runtime's zero-based row
// and byte column. go/token counts 1-based byte columns too, so a
// finding a tree-sitter frontend reports at a byte prints the column
// a Go finding at the same byte prints.
//
// # Dependency position
//
// The package imports github.com/tree-sitter/go-tree-sitter,
// go.dokimi.dev/eidos/sdk/position and the Go stdlib. It and the
// grammar packages beneath it are the only importers of the binding
// and the grammars in the repository, which a lint rule enforces.
package treesitter
