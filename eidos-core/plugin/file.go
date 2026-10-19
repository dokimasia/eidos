// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"slices"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

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
	// Source is the identity of the source file that every unit of the
	// file derives from, and the zero identity for a file of a package, of
	// a plan, or of two or more source files.
	Source symbol.Identity
	// Annotations are the annotations of the source file, such as its
	// build constraint, which the target writes where its language
	// carries one over to a file that derives from the source.
	Annotations symbol.Annotations
	// Header is the file-level statement of the generators of the file,
	// which [HeaderOf] returns for its units. The zero Header states
	// nothing.
	Header emit.File
	// Referents are the packages of the other files that the layout
	// qualified a reference of the file into, each once and sorted by
	// package path. A target whose package clause does not follow from the
	// package path, such as protobuf, reads the clause of such a file from
	// the Name of its package.
	Referents []symbol.Identity
}

// HeaderOf returns the header that the emit.File declarations of units
// state: the documentation and the annotations of each, in unit order.
// An annotation that an earlier emit.File states alike is left out, and
// so is a documentation. One emit.File keeps every annotation that it
// states. The path of an emit.File is not part of the header.
//
// # Allocation contract
//
// HeaderOf visits each declaration once. The header of one emit.File
// shares its lists, capped so that a later append copies them, and
// allocates nothing. Each later emit.File allocates what the merged
// documentation and annotations grow by, and HeaderOf never writes into
// the lists of an emit.File.
func HeaderOf(units []Unit) emit.File {
	var h emit.File
	// earlier lists the emit.File declarations that the header took in,
	// in an array on the stack for the few that one file has.
	var stack [4]*emit.File
	earlier := stack[:0]
	for _, u := range units {
		for _, d := range u.Decls {
			x, header := d.(*emit.File)
			if !header {
				continue
			}
			if len(earlier) == 0 {
				h.Doc, h.Annotations = slices.Clip(x.Doc), slices.Clip(x.Annotations)
				earlier = append(earlier, x)
				continue
			}
			if !slices.ContainsFunc(earlier, func(e *emit.File) bool { return slices.Equal(e.Doc, x.Doc) }) {
				h.Doc = append(h.Doc, x.Doc...)
			}
			stated := len(h.Annotations)
			for _, a := range x.Annotations {
				alike := func(b symbol.Annotation) bool {
					return a.Name == b.Name && a.Assigned == b.Assigned && slices.Equal(a.Args, b.Args)
				}
				if !slices.ContainsFunc(h.Annotations[:stated], alike) {
					h.Annotations = append(h.Annotations, a)
				}
			}
			earlier = append(earlier, x)
		}
	}
	return h
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
