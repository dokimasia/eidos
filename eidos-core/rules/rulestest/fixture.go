// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"context"
	"io/fs"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// pluginSet is the plugin-set fingerprint every fixture load folds:
// a fixed value in place of the one a composed workspace computes.
const pluginSet = "rulestest"

// Loaded loads a source tree through one frontend into a fixture:
// the sealed graph, and a fact store with the load's classification
// stamps under the kernel's keys and every key the registrations
// add. The load runs under [frontendtest.Brand], so a fixture tree
// writes its carriers the way the frontend suite reads them. A load
// that reports an Error fails the test, because a tree the language
// refuses proves nothing about its rules.
func Loaded(
	tb assert.TB, f plugin.Frontend, sources fs.FS, keys ...func(*meta.Registry) error,
) *Fixture {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(context.Background(), load.Config{
		FS:        sources,
		Frontends: []plugin.Frontend{f},
		Sink:      sink,
		PluginSet: []byte(pluginSet),
		Brand:     frontendtest.Brand,
	})
	assert.NoError(tb, err, "the fixture tree loads")
	for d := range sink.All() {
		if d.Severity == diag.SeverityError {
			tb.Errorf("the fixture load reported %v", d)
		}
	}
	registry := meta.NewRegistry()
	kernel, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel's keys register")
	for _, register := range keys {
		assert.NoError(tb, register(registry), "the language's keys register")
	}
	facts := meta.NewFacts(registry)
	for id, stamps := range g.Stamps() {
		for i, s := range stamps {
			err := facts.StampRaw(s, meta.Claim{
				Subject: id, Authority: meta.AuthorityPlugin, Plugin: s.Origin, Seq: i, Pos: s.Pos,
			})
			assert.NoError(tb, err, "a load stamp applies")
		}
	}
	return &Fixture{Graph: g, Facts: facts, Keys: kernel}
}
