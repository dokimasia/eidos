// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix opens every refusal of the backend: the language's
// identity, as every satellite's refusals open.
const refusalPrefix = string(protobuf.Lang) + ": "

// featuresPrefix opens the option of one feature, and
// [protobuf.FeaturesOption] alone sets features through a message
// literal.
const featuresPrefix = protobuf.FeaturesOption + "."

// Scope is the context of the declarations of one body of a protobuf
// file: the version of the file, the resolved features that apply to the
// declarations of the body, and whether the body is inside a message. The
// templates read it to decide how a construct is written in the version of
// its file, such as the label of a field, and which feature a declaration
// states where the resolved value differs from what its shape needs.
//
// A Scope resolves features as the protobuf frontend resolves them: from
// the defaults of the version, through the features of the header, of each
// enclosing message, of a oneof and of the declaration itself, in that
// order.
//
// The zero Scope is no resolution. [FileScope] returns the scope of the top
// level of a file, and [Scope.Within] the scope inside a declaration.
//
// # Concurrency
//
// A Scope is a value, and every method reads it alone.
//
// # Allocation contract
//
// No method allocates. A refusal allocates its error.
type Scope struct {
	version  protobuf.Edition
	features protobuf.Features
	nested   bool
}

// FileScope returns the scope of the top level of the file whose header is
// h: the version that its marker [protobuf.SyntaxMarker] states, and the
// defaults of that version with the features of the header applied. A
// header without the marker states proto3, the version that every protobuf
// toolchain reads.
//
// Error modes, each a refusal of the header:
//   - two syntax markers of different versions;
//   - a syntax marker without exactly one argument;
//   - a version outside the table of [protobuf.EditionNamed];
//   - a feature in a file of proto2 or proto3, which have no features;
//   - a feature without exactly one argument.
func FileScope(h *emit.File) (Scope, error) {
	name := protobuf.Proto3
	stated := false
	for _, a := range h.Annotations {
		if a.Name != string(protobuf.SyntaxMarker) {
			continue
		}
		switch {
		case len(a.Args) != 1:
			return Scope{}, fmt.Errorf(refusalPrefix+"the marker %s takes one argument, and it states %d",
				a.Name, len(a.Args))
		case stated && a.Args[0] != name:
			return Scope{}, fmt.Errorf(refusalPrefix+"the header states the versions %s and %s, "+
				"and a file has one version", name, a.Args[0])
		}
		name, stated = a.Args[0], true
	}
	version, known := protobuf.EditionNamed(name)
	if !known {
		return Scope{}, fmt.Errorf(refusalPrefix+"the header states the version %s, which protobuf does not have",
			name)
	}
	s := Scope{version: version, features: version.Defaults}
	return s.applied(h.Annotations, "the header")
}

// Within returns the scope inside a declaration that states the
// annotations as: s with the features of as applied, inside a message. A
// message, a oneof and an enum pass their annotations, so their members
// read the features that they state.
//
// Error modes: a feature in a file of proto2 or proto3, and a feature
// without exactly one argument, each a refusal of the declaration.
func (s Scope) Within(as symbol.Annotations) (Scope, error) {
	inner := s
	inner.nested = true
	return inner.applied(as, "a declaration")
}

// Version returns the protobuf version of the file.
func (s Scope) Version() protobuf.Edition { return s.version }

// Features returns the features that apply to the declarations of the
// body.
func (s Scope) Features() protobuf.Features { return s.features }

// Nested reports whether the body is inside a message, so a declaration of
// the body is a nested declaration.
func (s Scope) Nested() bool { return s.nested }

// applied returns s with the features that the annotations as state
// applied in order. who names the subject of a refusal.
func (s Scope) applied(as symbol.Annotations, who string) (Scope, error) {
	for _, a := range as {
		if a.Name != protobuf.FeaturesOption && !strings.HasPrefix(a.Name, featuresPrefix) {
			continue
		}
		switch {
		case s.version.Syntax:
			return Scope{}, fmt.Errorf(refusalPrefix+"%s states the feature %s, and %s has no features",
				who, a.Name, s.version.Name)
		case len(a.Args) != 1:
			return Scope{}, fmt.Errorf(refusalPrefix+"the feature %s takes one argument, and it states %d",
				a.Name, len(a.Args))
		}
		s.features = s.features.Apply(a.Name, a.Args[0])
	}
	return s, nil
}
