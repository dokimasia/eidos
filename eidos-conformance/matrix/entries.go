// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package matrix

import (
	"go.dokimi.dev/eidos/conformance"
	gocorpus "go.dokimi.dev/eidos/conformance/lang/go"
	javacorpus "go.dokimi.dev/eidos/conformance/lang/java"
	protocorpus "go.dokimi.dev/eidos/conformance/lang/protobuf"
	rustcorpus "go.dokimi.dev/eidos/conformance/lang/rust"
	tscorpus "go.dokimi.dev/eidos/conformance/lang/typescript"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	javabackend "go.dokimi.dev/eidos/lang/java/backend"
	rustbackend "go.dokimi.dev/eidos/lang/rust/backend"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The brackets in which the targets write the type arguments of a
// reference.
const (
	squareOpen  = "["
	squareClose = "]"
	angleOpen   = "<"
	angleClose  = ">"
)

// The markers of the brand acme in the languages with markers.
const (
	decoratorMarker  = "@acme.stub"
	attributeMarker  = "#[acme::stub]"
	annotationMarker = "@acme.stub"
)

// Entry is one language of the support matrix.
type Entry struct {
	// Corpus is the language's conformance entry. Its coverage contains
	// the verdicts of the read side and of the sugar.
	Corpus conformance.Corpus
	// Backend is the language's rendering backend, and nil for a
	// language without one. The target of the backend is the column of
	// the language on the render side and in the hub. The coverage and the
	// refused kinds of the backend fill the render side, and its spoke
	// fills the hub.
	Backend plugin.Backend
	// ArgsOpen and ArgsClose are the brackets in which the target writes
	// the type arguments of a reference, such as < and >.
	ArgsOpen, ArgsClose string
	// Marker is the spelling of a marker of the brand acme, such as
	// @acme.stub, and empty for a language without markers.
	Marker string
}

// Entries returns the entry of each satellite, in the order Go,
// TypeScript, Java, Rust and protobuf. The matrix reads only the coverage
// of each corpus, so a corpus does not have a tree.
func Entries() []Entry {
	return []Entry{
		{Corpus: gocorpus.Corpus(nil), Backend: gobackend.New(), ArgsOpen: squareOpen, ArgsClose: squareClose},
		{
			Corpus: tscorpus.Corpus(nil), Backend: tsbackend.New(), ArgsOpen: angleOpen, ArgsClose: angleClose,
			Marker: decoratorMarker,
		},
		{
			Corpus: javacorpus.Corpus(nil), Backend: javabackend.New(), ArgsOpen: angleOpen, ArgsClose: angleClose,
			Marker: annotationMarker,
		},
		{
			Corpus: rustcorpus.Corpus(nil), Backend: rustbackend.New(), ArgsOpen: angleOpen, ArgsClose: angleClose,
			Marker: attributeMarker,
		},
		{Corpus: protocorpus.Corpus(nil)},
	}
}
