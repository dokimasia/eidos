// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest_test

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The scripted trees of the memo cases. Every fixture of the test binary
// shares the memo, so each case loads a file that no other case of the
// binary loads.
const (
	onceFile   = "memo/once/once.zz"
	onceSource = "package memo/once\ntype Once string\n"
	ownFile    = "memo/own/own.zz"
	ownPackage = "memo/own"
	ownSource  = "package memo/own\ntype Own string\n"
	strayFile  = "memo/stray/stray.zz"
	straySrc   = "type Stray string\n"
)

// countingFront is the scripted frontend that counts the units that it
// parses.
type countingFront struct {
	*frontendtest.Scripted
	parses *atomic.Int64
}

// Parse counts the unit, then parses it as the scripted frontend does.
func (f countingFront) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	f.parses.Add(1)
	return f.Scripted.Parse(ctx, u)
}

// The fixtures of a test binary share the parse of a tree, and each
// fixture has declarations of its own.
func TestMemo(t *testing.T) {
	t.Parallel()

	t.Run("Loaded", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a unit once for two fixtures of one tree", func(t *testing.T) {
			t.Parallel()

			parses := &atomic.Int64{}
			s := plugintest.Source{
				Tree:      fstest.MapFS{onceFile: {Data: []byte(onceSource)}},
				Brand:     frontendtest.Brand,
				Frontends: []plugin.Frontend{countingFront{frontendtest.NewScripted(), parses}},
			}
			plugintest.Loaded(t, s)
			plugintest.Loaded(t, s)
			assert.Equal(t, parses.Load(), int64(1), "the second fixture restores the unit that the first parsed")
		})

		t.Run("gives each fixture declarations of its own", func(t *testing.T) {
			t.Parallel()

			s := plugintest.Source{
				Tree:      fstest.MapFS{ownFile: {Data: []byte(ownSource)}},
				Brand:     frontendtest.Brand,
				Frontends: []plugin.Frontend{frontendtest.NewScripted()},
			}
			own := symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: ownPackage, Name: "Own", Kind: symbol.KindStruct,
			}
			first, second := plugintest.Loaded(t, s), plugintest.Loaded(t, s)
			first.Graph.Freeze()
			second.Graph.Freeze()
			parsed, held := first.Graph.Lookup(own)
			assert.True(t, held, "the first fixture has Own")
			restored, held := second.Graph.Lookup(own)
			assert.True(t, held, "the second fixture has Own")
			assert.That(t, restored).
				Equal(parsed, "the restored declaration has the value of the parsed one").
				NotEqual(parsed, "the fixtures share no declaration", assert.ByIdentity())
		})

		t.Run("fails an Error of the load that a restored unit replays", func(t *testing.T) {
			t.Parallel()

			parses := &atomic.Int64{}
			s := plugintest.Source{
				Tree:      fstest.MapFS{strayFile: {Data: []byte(straySrc)}},
				Brand:     frontendtest.Brand,
				Frontends: []plugin.Frontend{countingFront{frontendtest.NewScripted(), parses}},
			}
			for range 2 {
				records := assert.Rejects(t, "a file without a package line", func(tb assert.TB) {
					plugintest.Loaded(tb, s)
				})
				assert.NotEmpty(t, records, "the load of each fixture fails")
			}
			assert.Equal(t, parses.Load(), int64(1), "the second fixture replays the finding of the restored unit")
		})
	})
}
