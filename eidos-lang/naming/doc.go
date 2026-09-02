// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

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
//
// Each delegates through a [Caser] holding the initialisms it
// recognises. That recognition is what lets snake round-trip
// through Pascal without losing an acronym's shape: "url_path"
// becomes "URLPath" rather than "UrlPath". The package-level
// functions use a [Default] Caser carrying [CommonInitialisms];
// a consumer needing another set builds one with [New] and
// [Caser.WithInitialisms], which is how a language whose acronym
// vocabulary differs configures its own.
//
// [IsIdentifier] is a separate concern: it reports the ASCII
// identifier shape, which a backend refusing to respell wire
// names tests before any convention runs. [SnakeFilename] joins
// the filename shape the snake-cased languages share.
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
// lang/naming imports the Go stdlib and nothing else, so importing
// it pulls none of the module's grammar machinery. It knows no
// single language: which spellings a target reserves, and which
// style it names files or types in, are each satellite's own
// knowledge.
package naming
