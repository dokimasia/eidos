// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// bindings is what a file's parse records for its own Resolve: the
// proto package its declarations load under.
//
// A proto import names a file, not a namespace, and a file's
// namespace is known only once that file parsed, so a reference
// probes the namespaces the workspace declares and not the paths a
// file imports. The imports are stamped for a consumer that checks
// them.
type bindings struct {
	pkg string
}

// Resolve returns what a type spelling could mean from where it is
// written, as [protobuf.Candidates] returns it: one tier per scope,
// innermost first, so the resolution phase binds the innermost
// declaration of a name and an outer one of the same name raises no
// ambiguity.
//
// The scope inside a message is the message's own chain. A oneof is
// no scope in protobuf, because its members are fields of the
// message, so a reference inside a oneof resolves in the message
// that declares the oneof. A scalar and a well-known type return no
// candidate and keep their spellings for the rules to classify.
//
// The probe covers every namespace the workspace declares, not only
// the imported ones, so a reference protoc would reject for a
// missing import resolves here: the read side reports what the graph
// contains, and validating imports is protoc's.
func (protoFrontend) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	pkg := ""
	if b, recorded := scope.Bindings.(*bindings); recorded && b != nil {
		pkg = b.pkg
	}
	return protobuf.Candidates(pkg, chainOf(scope.Owner), spelling)
}

// chainOf returns the chain of messages a reference is written in:
// a message's own chain, and the chain of the message that declares
// a oneof, an enum or a service, none of which is a scope a type
// resolves in.
func chainOf(owner symbol.Identity) string {
	if owner.Kind != symbol.KindStruct {
		return owner.Owner
	}
	if owner.Owner == "" {
		return owner.Name
	}
	return owner.Owner + protobuf.NameSep + owner.Name
}

// BindingsFor returns the opaque resolution record of a file in one
// proto package, the value the load hands Resolve as
// [plugin.ImportScope.Bindings].
//
// It exists for a caller reading the candidate order without a
// loaded tree: a test, or a tool auditing how a spelling would
// resolve. A load never calls it, because the parse records the
// bindings itself.
func BindingsFor(pkg string) any { return &bindings{pkg: pkg} }
