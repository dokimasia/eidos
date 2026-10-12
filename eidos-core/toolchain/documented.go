// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/plugin"
)

// AssertDocumented requires a doc comment on every public declaration at
// file level of the files of g that the frontend f claims. It loads the
// files under the context of tb, as [Surface] loads them. A method whose
// host is not public is outside the check. A member of a host, such as a
// field or a method in the body of a class, needs no doc comment of its
// own.
//
// Each declaration without a doc comment fails the test on a record of
// its own, with its kind, its name and the path of its file. The name of
// a method is the name of its host, a dot and its own name.
// AssertDocumented stops the test where f is nil, where the load fails or
// reports an Error, and where the files of g declare nothing public.
func AssertDocumented(tb assert.TB, f plugin.Frontend, g Generated) {
	tb.Helper()

	assert.NotNil(tb, f, frontendContract)
	lang := string(f.Lang())
	decls, err := loadOutput(assert.Context(tb), f, g)
	assert.NoError(tb, err, lang+loadsContract)
	assert.NotEmpty(tb, decls, lang+publicContract)
	for _, d := range decls {
		expect.NotEmpty(tb, d.decl.Docs(),
			lang+": the public "+d.decl.Kind().String()+" "+d.name+" in "+d.file+" has a doc comment")
	}
}
