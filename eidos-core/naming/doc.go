// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package naming converts an identifier between the case
// conventions target languages spell their names in.
//
// [Words] is the primitive: it splits an identifier into its
// component words, recognising camelCase and PascalCase
// boundaries, acronym runs (HTTPServer becomes HTTP and Server),
// and the separator characters (_, -, ., space, slash, tab).
// Every style is built on it.
//
// # The styles
//
//   - [Pascal] spells PascalCase
//   - [Camel] spells camelCase
//   - [Snake] spells snake_case
//   - [ScreamingSnake] spells SCREAMING_SNAKE_CASE
//   - [Kebab] spells kebab-case
//   - [ScreamingKebab] spells SCREAMING-KEBAB-CASE
//   - [Dot] spells dot.case
//   - [Title] spells Title Case
//
// Each delegates through a [Caser] holding the initialisms it
// recognises. That recognition is what lets snake round-trip
// through Pascal without losing an acronym's shape: "url_path"
// becomes "URLPath" rather than "UrlPath". The package-level
// functions use a [Default] Caser carrying [CommonInitialisms];
// a consumer needing another set builds one with [New] and
// [Caser.WithInitialisms].
//
// [Identifier] is a separate concern: it sanitises arbitrary text
// into a spelling the C-family languages accept, without changing
// case.
//
// # Allocation contract
//
// [Words] returns substrings of its input, so it allocates the
// slice and nothing else; a word carrying invalid UTF-8 is the one
// exception, rebuilt with U+FFFD per invalid byte. Every style
// writes through one Builder, so a conversion costs one
// allocation for the result. Recognising an initialism costs
// none: the probe upper-cases into a stack buffer.
//
// # Dependency position
//
// core/naming imports the Go stdlib and nothing else. It knows no
// language: which spellings a target reserves, and which style it
// names files or types in, are the satellite's own knowledge.
package naming
