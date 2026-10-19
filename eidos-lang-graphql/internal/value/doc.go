// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package value parses the values of GraphQL for the rules and the backend
// of the satellite, so both read a value through one grammar: the grammar
// of a constant value of the September 2025 edition. [Parse] parses one
// value through the lexer of gqlparser and returns it in the abstract syntax
// of gqlparser, which the frontend reads too.
//
// # Dependency position
//
// lang/graphql/internal/value imports the ast and lexer packages of
// gqlparser and the Go stdlib.
package value
