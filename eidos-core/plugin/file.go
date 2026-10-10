// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import "go.dokimi.dev/eidos/core/symbol"

// File is one output file a plan writes: where it is written, the
// package it declares, and the units it assembles, in render order.
// The plan's layout composes the files after the settle, and the
// render pass renders each one.
type File struct {
	// Path is workspace-relative and slash-separated: where the file
	// is written, and the position the render reports its findings at.
	Path string
	// Pkg is the package the file declares, with Name set to the name
	// the target writes in its package clause. The zero identity means
	// the target derives no package for the path.
	Pkg symbol.Identity
	// Units are the file's units in the store's order. A unit that
	// routing split keeps its plugin, family and key, and contains
	// only the declarations written in this file.
	Units []Unit
}

// FileSpeller is a target's filename half: how a unit splits into the
// files the target writes and what each file is named. The layout
// calls it on one goroutine.
type FileSpeller interface {
	// SplitUnit reshapes one unit into the units the target writes as
	// separate files, such as one unit per public type, and returns
	// the unit whole where the target writes it as one file. A unit
	// with declarations splits into at least one unit, and the split
	// keeps every declaration.
	SplitUnit(u Unit) []Unit
	// FileName spells one unit's filename from its cardinality, its
	// [Unit.FileKey], its family word and its tag, in the target's case
	// and join, with the target's extension. A target reads the unit's
	// package too where its language spells a package's files apart, as
	// Go names every file of an external test package _test.go.
	FileName(u Unit) string
}
