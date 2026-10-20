// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// New returns the Python backend that writes modules, files with the
// extension .py, for the version of Python that the policy python.version
// selects: the module's declared pieces composed through the kernel's kit
// into a [backend.Backend]. Its spoke is [spell.Type], which spells a type
// of another language under the target's lowering policies,
// [python.Policies]. A module writes one implementation after the overload
// signatures of a function name, and evaluates what [Evaluated] yields when
// it runs. Rendered files finalise through [Layout.Format], which lays them
// out as ruff's formatter does, because no formatter of Python is a library
// of Go and hermeticity refuses a formatter that the machine provides.
//
// Where the table of the standard library of typeshed, which the module
// embeds, does not parse, every module fails to start with the error of the
// table.
func New() backend.Backend {
	return compose(typeshed.VersionsText, spell.Filename, false)
}

// Stubs returns the Python backend that writes stub files, files with the
// extension .pyi, as [New] composes the backend of modules, with
// [LowerStub] as its lowering. Every callable has the body ..., and every
// default and every value is ..., except the value of a constant and of a
// member of an enum, which is part of its type.
// A stub file writes no annotation as a string, because a type checker reads
// a stub without running it, and it has the blank lines that ruff's
// formatter writes in a stub file. For the same reason a stub file keeps the
// order of the flush, and it states overload signatures without their
// implementation. A table of typeshed that does not parse fails every stub
// file as it fails every module of New.
func Stubs() backend.Backend {
	return compose(typeshed.VersionsText, spell.StubFilename, true)
}

// stubVersion follows the version of the output in the version of the
// backend of stub files. The two backends share their name, so a plan that
// changes from one to the other changes the fingerprint of its composition
// through the version alone.
const stubVersion = "; stub"

// compose builds the backend through the kit with the standard library of
// the table of typeshed versions, naming the files of units through naming,
// writing stubs where stub is true. The importer keeps the error of a table
// that does not parse. A module is implemented and evaluates its
// declarations, and a stub file is neither.
func compose(versionsText []byte, naming func(plugin.Unit) string, stub bool) backend.Backend {
	versions, err := typeshed.ParseVersions(versionsText)
	im := importer{versions: versions, err: err}
	version, lower := python.BackendVersion, Lower
	if stub {
		version, lower = version+stubVersion, LowerStub
	}
	b := backend.New(python.Name, python.Target, python.Syntax()).
		Version(version).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		RefusedKinds(RefusedKinds()).
		Coverage(Coverage()).
		Funcs(vocabulary(im, stub)).
		Naming(naming).
		Packages(spell.Package).
		Overloads().
		Respell(spell.Name).
		RespellMember(spell.Member).
		Types(spell.Type).
		Policies(python.Policies()...).
		Lower(lower).
		Scaffold(Scaffold).
		Value(Value).
		MemberIndent(python.Indent).
		Imports(im.Imports).
		TemplateImport(TemplateImport).
		Finalise(Layout{Stub: stub}.Format)
	if !stub {
		b.Implemented().Evaluated(Evaluated)
	}
	return b.Build()
}
