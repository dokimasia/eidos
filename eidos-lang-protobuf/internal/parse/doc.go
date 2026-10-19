// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package parse parses proto sources for the frontend, the backend and the
// toolchain adapter of the satellite, so the three read a source of each
// protobuf version through one grammar. [File] parses one source with the
// experimental parser of protocompile, and parses a file of edition 2026
// from a copy that states edition 2024.
//
// # Dependency position
//
// lang/protobuf/internal/parse imports the experimental ast, parser,
// report, source and keyword packages of protocompile, the module root for
// the names of the editions, and the Go stdlib.
package parse
