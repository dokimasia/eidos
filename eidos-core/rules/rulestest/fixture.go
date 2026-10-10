// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"context"
	"io/fs"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// Loaded loads a source tree through one frontend into a fixture:
// the sealed graph, and a fact store with the load's classification
// stamps under the kernel's keys, the keys that the frontend registers
// through its role, and every key the registrations add. The frontend
// registers under its language's spelling and the registrations under
// the handle of the composition, as a workspace registers them. The
// load runs under [frontendtest.Brand], so a fixture tree writes its
// carriers the way the frontend suite reads them. A load that reports
// an Error fails the test, because a tree the language refuses proves
// nothing about its rules.
func Loaded(
	tb assert.TB, f plugin.Frontend, sources fs.FS, keys ...func(*meta.Registry) error,
) *Fixture {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(context.Background(), load.Config{
		FS:        sources,
		Frontends: []plugin.Frontend{f},
		Sink:      sink,
		Brand:     frontendtest.Brand,
	})
	assert.NoError(tb, err, "the fixture tree loads")
	for d := range sink.All() {
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			"the fixture load reports no Error: "+d.Code.String()+" "+d.Msg)
	}
	registry := meta.NewRegistry()
	kernel, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel's keys register")
	if kp, provides := f.(plugin.KeyProvider); provides {
		assert.NoError(tb, kp.Keys(registry.For(string(f.Lang()))), "the frontend's keys register")
	}
	for _, register := range keys {
		assert.NoError(tb, register(registry), "the fixture's keys register")
	}
	facts := meta.NewFacts(registry)
	for id, stamps := range g.Stamps() {
		for i, s := range stamps {
			err := facts.StampRaw(s, meta.Claim{
				Subject:   id,
				Authority: meta.AuthorityPlugin,
				Plugin:    s.Origin,
				Order:     meta.Order{Subject: id, Instance: i},
				Pos:       s.Pos,
			})
			assert.NoError(tb, err, "a load stamp applies")
		}
	}
	return &Fixture{Graph: g, Facts: facts, Keys: kernel}
}
