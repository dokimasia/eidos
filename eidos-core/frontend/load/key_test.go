// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// keysOf maps each unit's first member onto its key.
func keysOf(report *load.Report) map[string][]byte {
	out := make(map[string][]byte, len(report.Units))
	for _, u := range report.Units {
		out[u.Files[0].Path] = u.Key
	}
	return out
}

// keyed loads the standard tree, mutated per case, and returns its
// unit keys.
func keyed(tb assert.TB, mutate ...func(*load.Config)) map[string][]byte {
	tb.Helper()

	_, report, _ := loadTree(tb, stdTree(), mutate...)
	return keysOf(report)
}

// Every part of the stated fold is exercised: an untouched unit's
// key is stable, and each folded part changes it alone.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("unitKey", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the same key for the same load", func(t *testing.T) {
			t.Parallel()

			one, two := keyed(t), keyed(t)
			assert.Equal(t, len(one), 3, "three units load")
			for file, key := range one {
				assert.True(t, bytes.Equal(key, two[file]), "the key is stable")
			}
		})

		changedRead := func(tb assert.TB) map[string][]byte {
			tb.Helper()

			tree := stdTree()
			tree[apiFile] = &fstest.MapFile{Data: []byte("package svc/api\ntype User int\n")}
			_, report, _ := loadTree(tb, tree)
			return keysOf(report)
		}

		t.Run("changes the key of a unit whose read changed", func(t *testing.T) {
			t.Parallel()

			assert.False(t, bytes.Equal(keyed(t)[apiFile], changedRead(t)[apiFile]), "the unit's key changes")
		})

		t.Run("keeps the key of a unit whose reads did not change", func(t *testing.T) {
			t.Parallel()

			assert.True(t, bytes.Equal(keyed(t)[storeFile], changedRead(t)[storeFile]), "the unit's key is stable")
		})

		t.Run("changes the key of every unit that read a changed shared input", func(t *testing.T) {
			t.Parallel()

			before := keyed(t)
			tree := stdTree()
			tree[modFile] = &fstest.MapFile{Data: []byte("mod v2\n")}
			_, report, _ := loadTree(t, tree)
			after := keysOf(report)
			for file := range before {
				assert.False(t, bytes.Equal(before[file], after[file]), "the partition read folds into every unit")
			}
		})

		folds := []struct {
			name   string
			file   string
			mutate func(*load.Config)
		}{
			{
				name:   "changes the key for another depth",
				file:   depFile,
				mutate: func(cfg *load.Config) { cfg.Signatures = nil },
			},
			{
				name: "changes the key for another frontend version",
				file: apiFile,
				mutate: func(cfg *load.Config) {
					f := frontendtest.NewScripted()
					f.Ver = "2"
					cfg.Frontends = []plugin.Frontend{f}
				},
			},
			{
				name: "changes the key for other options",
				file: apiFile,
				mutate: func(cfg *load.Config) {
					f := frontendtest.NewScripted()
					f.Opts.Tag = "moved"
					cfg.Frontends = []plugin.Frontend{f}
				},
			},
			{
				name: "changes the key for another frontend name",
				file: apiFile,
				mutate: func(cfg *load.Config) {
					f := frontendtest.NewScripted()
					f.ID = "fake2"
					cfg.Frontends = []plugin.Frontend{f}
				},
			},
			{
				name:   "changes the key for another brand",
				file:   apiFile,
				mutate: func(cfg *load.Config) { cfg.Brand = foreignBrand },
			},
		}
		for _, tt := range folds {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.False(t, bytes.Equal(keyed(t)[tt.file], keyed(t, tt.mutate)[tt.file]),
					"the folded part changes the key")
			})
		}
	})
}
