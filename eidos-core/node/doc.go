// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package node is the declaration model a frontend produces from
// source.
//
// It declares the same kinds as the emit model and differs in what
// wraps them: a node declaration contains its source position and its
// canonical identity, and its member lists are plain slices,
// because the read side is sealed once loading finishes.
//
// Every kind returns [Declaration], which is [symbol.Symbol] plus
// the identity of this side. [Declarations] is the traversal typed by
// it, for a caller that keys on identity and does not walk neutrally.
//
// The kinds, [Walk], [All] and [Declarations] generate from the
// symbol schema. Editing a generated file fails the build, because
// the mirror guard reruns the generator and compares. [Imports] is
// hand-written: a package's import list is the union of its files',
// derived here and stored nowhere.
//
// # Encodings
//
// [EncodeJSON] and [DecodeJSON] write and read a declaration as JSON.
// [AppendBinary] and [DecodeBinary] write and read its subtree in the
// binary form a sealed state stores: each string a number in the
// region's [StringTable], or, without a table, its length and bytes,
// which makes two equal subtrees equal bytes. Both codecs generate from
// the schema, so a kind the schema adds encodes without a hand edit.
// Every failure to read a binary encoding wraps [ErrMalformed].
//
// # Dependency position
//
// core/node imports core/symbol, core/position, core/internal/wire and
// the Go stdlib. It never imports core/emit: origin points one way, so
// the read side cannot observe the write side.
package node
