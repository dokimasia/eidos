// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import "go.dokimi.dev/eidos/sdk/symbol"

// MarkerPrefix opens the name of every [Marker]. A Python name cannot
// contain a colon, so no decorator has the name of a marker.
const MarkerPrefix = "python:"

// Marker is the name of an annotation that states a construct of Python
// that the model has no field for. A generator states a marker as an
// annotation with the name of the marker, and the backend writes the
// construct for it and no decorator.
type Marker string

// The markers of the Python target.
const (
	// TypedDictMarker marks an interface as a TypedDict.
	TypedDictMarker Marker = MarkerPrefix + "typeddict"
	// BasesMarker states the bases of an enum, one argument for each base,
	// such as IntFlag.
	BasesMarker Marker = MarkerPrefix + "bases"
	// KeywordsMarker states the keyword arguments of a class statement, one
	// argument for each keyword, such as total=False.
	KeywordsMarker Marker = MarkerPrefix + "keywords"
	// AnnotatedMarker states the metadata of the Annotated type of a field,
	// a parameter, a variable, a constant or an alias, one argument for
	// each element.
	AnnotatedMarker Marker = MarkerPrefix + "annotated"
	// InitVarMarker marks a field of a dataclass whose type is InitVar[T],
	// which the generated __init__ takes and the instance does not keep.
	InitVarMarker Marker = MarkerPrefix + "initvar"
	// AllMarker is a header marker. Its arguments are the names of the
	// __all__ of the file.
	AllMarker Marker = MarkerPrefix + "all"
	// LazyMarker is a header marker. Its arguments are the modules that the
	// file imports with the keyword lazy.
	LazyMarker Marker = MarkerPrefix + "lazy"
	// FutureMarker is a header marker. Its arguments are the features of the
	// future statement of the file.
	FutureMarker Marker = MarkerPrefix + "future"
	// HeadMarker is a header marker. Its arguments are the comments of the
	// head of the file, each without its #.
	HeadMarker Marker = MarkerPrefix + "head"
	// ImportMarker is a header marker. Its arguments are the imports of the
	// file, one statement for each name that an import binds, such as
	// from dataclasses import field, import json and from shop import *.
	// The backend writes each in the import block of the file, so the
	// names that a value or a decorator of the file uses are bound.
	ImportMarker Marker = MarkerPrefix + "import"
)

// Valid reports whether m is one of the markers that the satellite
// declares. The backend refuses an annotation whose name starts with
// [MarkerPrefix] and is not a valid marker. Valid does not allocate.
func (m Marker) Valid() bool {
	switch m {
	case TypedDictMarker, BasesMarker, KeywordsMarker, AnnotatedMarker, InitVarMarker, AllMarker, LazyMarker,
		FutureMarker, HeadMarker, ImportMarker:
		return true
	default:
		return false
	}
}

// Header reports whether m is a marker of the header of a file, which the
// backend writes into the head of the module. Header does not allocate.
func (m Marker) Header() bool {
	switch m {
	case AllMarker, LazyMarker, FutureMarker, HeadMarker, ImportMarker:
		return true
	default:
		return false
	}
}

// Marks reports whether m is a marker of a declaration of the kind k: the
// TypedDict marker of an interface, the bases marker of an enum, the
// keywords marker of a struct, an interface or an enum, the annotated
// marker of a field, a parameter, a variable, a constant or an alias, the
// InitVar marker of a field, and a header marker of a file. The backend
// refuses a marker on a declaration of another kind. Marks reports false
// for a marker that the satellite does not declare, and does not allocate.
func (m Marker) Marks(k symbol.Kind) bool {
	switch m {
	case TypedDictMarker:
		return k == symbol.KindInterface
	case BasesMarker:
		return k == symbol.KindEnum
	case KeywordsMarker:
		return k == symbol.KindStruct || k == symbol.KindInterface || k == symbol.KindEnum
	case AnnotatedMarker:
		return k == symbol.KindField || k == symbol.KindParam || k == symbol.KindVariable ||
			k == symbol.KindConstant || k == symbol.KindAlias
	case InitVarMarker:
		return k == symbol.KindField
	case AllMarker, LazyMarker, FutureMarker, HeadMarker, ImportMarker:
		return k == symbol.KindFile
	default:
		return false
	}
}
