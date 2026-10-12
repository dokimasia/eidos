// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"io/fs"
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// Loaded loads a source tree through one frontend into a fixture. The
// fixture has the sealed graph, the findings of the load, and a fact
// store with the classification stamps of the load. The store registers
// its keys through [frontendtest.Registry], so a nil function of keys
// registers nothing. Each stamp has the claim of [meta.LoadClaim], as in
// a workspace run. The load runs under [frontendtest.Brand], so a
// fixture tree writes its carriers the way the frontend suite reads
// them, and under the context of tb, which ends with the test. A load
// that reports an Error fails the test, because a tree that the language
// refuses proves nothing about its rules.
func Loaded(
	tb assert.TB, f plugin.Frontend, sources fs.FS, keys ...func(*meta.Registry) error,
) *Fixture {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(assert.Context(tb), load.Config{
		FS:        sources,
		Frontends: []plugin.Frontend{f},
		Sink:      sink,
		Brand:     frontendtest.Brand,
	})
	assert.NoError(tb, err, "the fixture tree loads")
	findings := slices.Collect(sink.All())
	for _, d := range findings {
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			"the fixture load reports no Error: "+d.Code.String()+" "+d.Msg)
	}
	registry, kernel := frontendtest.Registry(tb, f, keys...)
	facts := meta.NewFacts(registry)
	for id, stamps := range g.Stamps() {
		for i, s := range stamps {
			assert.NoError(tb, facts.StampRaw(s, meta.LoadClaim(id, i, s)), "a load stamp applies")
		}
	}
	return &Fixture{Graph: g, Facts: facts, Keys: kernel, Findings: findings}
}
