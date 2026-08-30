// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package node is the declaration model a frontend produces from
// source.
//
// It carries the same kinds as the emit model and differs in what
// wraps them: a node declaration holds its source position and its
// canonical identity, and its member lists are plain slices,
// because the read side is sealed once loading finishes.
//
// The kinds, [Walk], [All] and [RewireOwners] generate from the
// symbol schema. Editing a generated file fails the build, because
// the mirror guard reruns the generator and compares.
//
// # Dependency position
//
// core/node imports core/symbol, core/position and the Go stdlib.
// It never imports core/emit: origin points one way, so the read
// side cannot observe the write side.
package node
