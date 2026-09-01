// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontendtest"
	"go.dokimi.dev/eidos/core/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// keysOf maps each unit's first member onto its key.
func keysOf(report *load.Report) map[string][]byte {
	out := make(map[string][]byte, len(report.Units))
	for _, u := range report.Units {
		out[u.Files[0]] = u.Key
	}
	return out
}

// Every part of the stated fold is exercised: an untouched unit's
// key is stable, and each folded part changes it alone.
func TestKeys(t *testing.T) {
	t.Parallel()

	base := func(tb assert.TB, mutate ...func(*load.Config)) map[string][]byte {
		tb.Helper()
		_, report, _ := loadTree(tb, stdTree(), mutate...)
		return keysOf(report)
	}

	t.Run("the same load folds the same keys", func(t *testing.T) {
		t.Parallel()

		one, two := base(t), base(t)
		assert.Equal(t, len(one), 3, "three units load")
		for file, key := range one {
			assert.True(t, bytes.Equal(key, two[file]), "an untouched unit's key is stable")
		}
	})

	t.Run("a changed read changes that unit's key alone", func(t *testing.T) {
		t.Parallel()

		before := base(t)
		tree := stdTree()
		tree["svc/api/user.zz"] = &fstest.MapFile{Data: []byte("package svc/api\ntype User int\n")}
		_, report, _ := loadTree(t, tree)
		after := keysOf(report)

		assert.False(t, bytes.Equal(before["svc/api/user.zz"], after["svc/api/user.zz"]),
			"the touched unit re-keys")
		assert.True(t, bytes.Equal(before["svc/store/row.zz"], after["svc/store/row.zz"]),
			"an untouched unit does not")
	})

	t.Run("a changed shared input re-keys every unit that read it", func(t *testing.T) {
		t.Parallel()

		before := base(t)
		tree := stdTree()
		tree["mod.zz"] = &fstest.MapFile{Data: []byte("mod v2\n")}
		_, report, _ := loadTree(t, tree)
		after := keysOf(report)
		for file := range before {
			assert.False(t, bytes.Equal(before[file], after[file]),
				"the partition read folds into every unit")
		}
	})

	t.Run("depth, version, options and the plugin set each fold", func(t *testing.T) {
		t.Parallel()

		before := base(t)

		shallow := base(t, func(cfg *load.Config) { cfg.Signatures = nil })
		assert.False(t, bytes.Equal(before["svc/dep/dep.zz"], shallow["svc/dep/dep.zz"]),
			"the same bytes at two depths key differently")

		bumped := base(t, func(cfg *load.Config) {
			f := frontendtest.NewScripted()
			f.Ver = "2"
			cfg.Frontends = []plugin.Frontend{f}
		})
		assert.False(t, bytes.Equal(before["svc/api/user.zz"], bumped["svc/api/user.zz"]),
			"a declared version change re-keys")

		retagged := base(t, func(cfg *load.Config) {
			f := frontendtest.NewScripted()
			f.Opts.Tag = "moved"
			cfg.Frontends = []plugin.Frontend{f}
		})
		assert.False(t, bytes.Equal(before["svc/api/user.zz"], retagged["svc/api/user.zz"]),
			"a knob that changes no read still keys")

		reset := base(t, func(cfg *load.Config) { cfg.PluginSet = []byte("set-2") })
		assert.False(t, bytes.Equal(before["svc/api/user.zz"], reset["svc/api/user.zz"]),
			"the composition's fingerprint folds")
	})
}
