// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package model

// The struct-tag vocabulary the schema annotates fields with.
//
// The set is closed: lowering refuses a token outside it, naming
// the schema position, so the annotation language grows by kernel
// decision rather than by typo.
const (
	// TagKey names the struct tag lowering reads.
	TagKey = "eidos"

	// TokenSeparator separates a tag's tokens.
	TokenSeparator = ","

	// SideNodeToken places a field on the node model only,
	// SideEmitToken on the emit model only, and SideBothToken on
	// both. One of the three opens every tag.
	SideNodeToken = "node"
	SideEmitToken = "emit"
	SideBothToken = "both"

	// WalkToken includes a field in the generated traversal. Only
	// containment edges carry it, which is what keeps the walk a
	// tree over a cyclic graph.
	WalkToken = "walk"

	// SlotPrefix opens a slot declaration, "slot=fields". On the
	// emit side the field becomes slot storage with typed
	// accessors instead of a plain slice.
	SlotPrefix = "slot="
)

// Names the schema fixes, which lowering matches against.
const (
	// MarkerName is the schema's heterogeneous-field marker: a
	// field of this type holds any kind.
	MarkerName = "Symbol"

	// HostField names the owning declaration of an owned kind. It
	// holds an identity rather than a pointer, so it is resolved
	// through a tracked read like any other cross-reference.
	HostField = "Host"
)

// The package the schema lives in, and the directory that holds it
// relative to the module root.
const (
	// SchemaPackage is the name lowering type-checks the schema
	// under.
	SchemaPackage = "schema"

	// SchemaDir is the schema's module-relative directory.
	SchemaDir = "symbol/schema"
)
