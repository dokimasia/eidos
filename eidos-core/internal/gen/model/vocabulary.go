// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

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

	// NameToken marks a string field as a declared name, which the
	// generated respell traversal visits. Only a plain string
	// carries it: a name is one spelling, never a list.
	NameToken = "name"

	// FactPrefix opens a fact declaration, "fact=Async": the field
	// states the named fact when it is set, and the generated facts
	// traversal visits it. The value is the generated constant's
	// suffix, so it spells as an exported identifier. Only an
	// emit-visible field carries it, because coverage is a render
	// question.
	FactPrefix = "fact="

	// SubjectMark is the doc directive that makes a kind a dispatch
	// subject, written as its own "//eidos:subject" line in the
	// kind's documentation. A marked kind gets its generated Match
	// type and trigger constructor, and the line is stripped from
	// the rendered docs. The spelling here is the line after the
	// comment markers strip.
	SubjectMark = "eidos:subject"
)

// Names the schema fixes, which lowering matches against.
const (
	// MarkerName is the schema's heterogeneous-field marker: a
	// field of this type holds any kind.
	MarkerName = "Symbol"

	// BodyMarkerName is the schema's body marker: a field of this
	// type carries a callable's emit-side content. The spelling
	// passes through bare, where it resolves to the emit package's
	// own Body value; the node model never sees the field, because
	// the schema declares it emit-side.
	BodyMarkerName = "Body"

	// annotationsType is the shared vocabulary's annotation list: a
	// field of this type carries the structured markers a
	// declaration holds, and it states its fact when non-empty,
	// which the symbol-qualified scalar rule would misjudge.
	annotationsType = "symbol.Annotations"
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
