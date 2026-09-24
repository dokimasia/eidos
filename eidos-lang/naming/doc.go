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
// Each style converts through a [Caser] and the initialisms it
// recognises, so "url_path" converts through Pascal to "URLPath" and
// back through Snake to "url_path".
// The package-level functions use the [Default] Caser, which
// recognises [CommonInitialisms]. A language whose acronym
// vocabulary differs builds its own Caser with [New] and
// [Caser.WithInitialisms].
//
// [IsIdentifier] reports the ASCII identifier shape, which a backend
// refusing to respell wire names tests before any convention runs.
// [FilenameParts] returns the parts every target builds a unit's
// filename from, and [SnakeFilename] joins them in the shape the
// snake-cased languages share.
//
// # Allocation contract
//
// [Words] returns substrings of its input, so it allocates the slice
// and nothing else. A word with invalid UTF-8 is the one exception:
// it is rebuilt with U+FFFD per invalid byte. Every style writes
// through one Builder, so a conversion allocates once for its result
// beside the words. Recognising an initialism allocates nothing,
// because the probe upper-cases into a stack buffer.
//
// # Dependency position
//
// lang/naming imports the Go stdlib and nothing else. It knows no
// single language: which spellings a target reserves, and which
// style it names files or types in, are each satellite's own
// knowledge.
package naming
