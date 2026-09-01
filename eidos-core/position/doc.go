// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package position holds source positions for declarations.
//
// [Pos] is the value every symbol carries: a workspace-relative,
// slash-separated file path on every platform, with 1-based line and
// column. The zero [Pos] means "no source position", which is what
// synthesized emit values carry.
//
// # Dependency position
//
// core/position imports only the Go stdlib.
package position
