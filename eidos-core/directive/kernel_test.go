// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
)

// The kernel schemas are the spellings consumers write in source,
// so their shape is API: names, params and repeatability all pin
// here.
func TestKernel(t *testing.T) {
	t.Parallel()

	// byName returns the kernel schema carrying a name.
	byName := func(tb assert.TB, name directive.Name) directive.Schema {
		tb.Helper()
		for _, s := range directive.Kernel() {
			if s.Name == name {
				return s
			}
		}
		assert.True(tb, false, "every kernel name returns a schema")
		return directive.Schema{}
	}

	t.Run("registers the four names cleanly", func(t *testing.T) {
		t.Parallel()

		r := directive.NewRegistry()
		schemas := directive.Kernel()
		assert.Length(t, schemas, 4, "meta, out, diag and skip")
		for _, s := range schemas {
			assert.NoError(t, r.Register(s), "every kernel schema passes its own registry")
			assert.Equal(t, s.Plugin, "", "and belongs to the kernel")
		}
		assert.Empty(t, r.Seal(), "and the set seals without faults")
	})

	t.Run("meta drops through the metadata registry", func(t *testing.T) {
		t.Parallel()

		s := byName(t, directive.KernelMeta)
		assert.True(t, s.Repeatable, "several facts drop on one subject")
		drop := s.Params[0]
		assert.Equal(t, drop.Key, directive.MetaDrop, "under the drop key")
		assert.Equal(t, drop.Resolution, directive.ResolveMetadataKey,
			"resolved against the metadata registry, so a typo names candidates")
	})

	t.Run("out declares the reserved routing keys as its own", func(t *testing.T) {
		t.Parallel()

		s := byName(t, directive.KernelOut)
		keys := map[directive.ParamKey]struct{}{}
		for _, spec := range s.Params {
			keys[spec.Key] = struct{}{}
		}
		_, path := keys[directive.OutPath]
		assert.True(t, path, "the redirect target")
		_, tag := keys[directive.OutTag]
		assert.True(t, tag,
			"and the companion selector: the kernel owns the reserved keys, "+
				"so its own schema may declare one")
	})

	t.Run("diag demands the code it suppresses", func(t *testing.T) {
		t.Parallel()

		s := byName(t, directive.KernelDiag)
		assert.True(t, s.Repeatable, "several codes suppress on one subject")
		off := s.Params[0]
		assert.Equal(t, off.Key, directive.DiagOff, "under the off key")
		assert.True(t, off.Required, "a suppression without a code suppresses nothing")
	})

	t.Run("skip narrows to one plugin only optionally", func(t *testing.T) {
		t.Parallel()

		s := byName(t, directive.KernelSkip)
		assert.False(t, s.Repeatable, "one exclusion per subject")
		plugin := s.Params[0]
		assert.Equal(t, plugin.Key, directive.SkipPlugin, "under the plugin key")
		assert.False(t, plugin.Required, "a bare skip excludes from everything")
	})
}
