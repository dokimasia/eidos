// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontendtest"
	"go.dokimi.dev/eidos/core/plugin"
)

// fixture is the scripted language's whole-contract tree: two
// packages, a cross-package reference, a builtin, members, a
// directive carrier, a classification stamp, a signature root and
// a broken file that reports.
func fixture() *frontendtest.Fixture {
	return &frontendtest.Fixture{
		Sources: fstest.MapFS{
			"mod.zz": {Data: []byte("mod v1\n")},
			"svc/api/user.zz": {Data: []byte(
				"package svc/api\ntype User string\nmethod Get int\n",
			)},
			"svc/store/row.zz": {Data: []byte(
				"package svc/store\nimport api svc/api\ntype Row api.User int\n+gen:table name=users\nconst rowmax\n",
			)},
			"svc/store/row_test.zz": {Data: []byte(
				"package svc/store\nstamp fake.testFile yes\ntype RowTest string\n",
			)},
			"svc/dep/dep.zz": {Data: []byte(
				"package svc/dep\ntype Dep string\nconst hidden\n",
			)},
			"bad/oops.zz": {Data: []byte("type Lost string\n")},
		},
		Signatures: []string{"svc/dep"},
		Schemas:    frontendtest.ScriptedSchemas(),
		Keys:       frontendtest.ScriptedKeys,
	}
}

// setup builds the scripted frontend over the whole-contract tree.
func setup(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontendtest.NewScripted(), fixture()
}

// The suite is the read side's conformance bar, so it must hold
// the scripted language — every check green over a fixture that
// states everything.
func TestRunFrontendSuite(t *testing.T) {
	t.Parallel()

	frontendtest.RunFrontendSuite(t, setup)
}
