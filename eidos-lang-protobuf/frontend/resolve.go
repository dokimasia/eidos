// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// pathSep separates the segments of an import path.
const pathSep = "/"

// bindings is what the parse of a file records for its own resolution and
// for [importPath]: the proto package under which its declarations load,
// and the import paths of the file, in source order.
//
// A proto import names a file, not a namespace, and the namespace of a
// file is known only once that file parsed. A reference therefore probes
// the namespaces that the workspace declares, and not the paths that a
// file imports. The imports name the file of the declaration of a resolved
// reference, which is what importPath reads.
type bindings struct {
	pkg     string
	imports []string
}

// importPath is the importer role of the frontend. It returns the import
// path among the imports of the referencing file that names the declaring
// file: the workspace path of the declaring file, or its suffix after a
// directory boundary where a proto root other than the workspace root
// places the import. It returns the first such import in source order. A
// declaring file that no import names, which protoc rejects, returns its
// workspace path. importPath allocates nothing.
func importPath(scope plugin.ImportScope, file string) string {
	if b, recorded := scope.Bindings.(*bindings); recorded && b != nil {
		for _, imported := range b.imports {
			if imported == file || strings.HasSuffix(file, pathSep+imported) {
				return imported
			}
		}
	}
	return file
}

// resolve returns what a type spelling could mean at the place where it is
// written, as [protobuf.Candidates] returns it: one tier per scope,
// innermost first, so the resolution phase binds the innermost declaration
// of a name, and an outer one of the same name raises no ambiguity.
//
// The scope inside a message is the own chain of the message. A oneof is
// no scope in protobuf, because its members are fields of the message, so
// a reference inside a oneof resolves in the message that declares the
// oneof. A scalar and a well-known type return no candidate and keep their
// spellings for the rules to classify.
//
// The probe covers every namespace that the workspace declares, not only
// the imported ones, so a reference that protoc would reject for a missing
// import resolves here. The read side reports what the graph contains, and
// the validation of imports belongs to protoc.
//
// resolve allocates what [protobuf.Candidates] allocates, and the joined
// chain of the scope of a nested message.
func resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	pkg := ""
	if b, recorded := scope.Bindings.(*bindings); recorded && b != nil {
		pkg = b.pkg
	}
	return protobuf.Candidates(pkg, chainOf(scope.Owner), spelling)
}

// chainOf returns the chain of messages in which a reference is written:
// the own chain of a message, and the chain of the message that declares a
// oneof, an enum or a service, none of which is a scope in which a type
// resolves.
func chainOf(owner symbol.Identity) string {
	if owner.Kind != symbol.KindStruct {
		return owner.Owner
	}
	if owner.Owner == "" {
		return owner.Name
	}
	return owner.Owner + protobuf.NameSep + owner.Name
}

// BindingsFor returns the opaque resolution record of a file in one proto
// package with the given imports, the value that the load hands the
// resolution and the importer role as [plugin.ImportScope.Bindings].
//
// It serves a caller that reads the order of the candidates without a
// loaded tree: a test, or a tool that audits how a spelling would resolve.
// A load never calls it, because the parse records the bindings itself.
//
// BindingsFor allocates the record, one allocation. The record keeps the
// list of the imports as it is, so a caller that lists the imports as
// arguments allocates the list too.
func BindingsFor(pkg string, imports ...string) any {
	return &bindings{pkg: pkg, imports: imports}
}
