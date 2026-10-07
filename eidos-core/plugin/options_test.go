// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/plugin"
)

// The allocations of the options checks over the lawful options, which
// TestOptionsAllocs checks in the ordinary run and BenchmarkOptions in a
// benchmark run.
const (
	// validateOptionsAllocs is the check of the lawful options: the set
	// of claimed keys with its first group, and the reflection of the
	// struct's two fields with the range over them.
	validateOptionsAllocs = 6
	// encodeOptionsAllocs is the encoding of the lawful options: the set
	// of seen types, the reflection of each field and the encoded bytes,
	// twelve allocations, and the six of encoding/json's state, which its
	// pool supplies until a collection empties the pool.
	encodeOptionsAllocs = 12 + 6
)

// optioned is a fixture plugin returning whatever options value it
// was built with.
type optioned struct {
	named
	cfg any
}

// Options returns the fixture's options value.
func (o optioned) Options() any { return o.cfg }

// lawfulOptions holds the tag contract whole: exported fields, an
// opt key and a doc on each.
type lawfulOptions struct {
	Header string `opt:"header" doc:"the banner every file opens with"`
	Depth  int    `opt:"depth"  doc:"how deep the mirror walks"`
}

// shadowedOptions hides one option from the composition.
type shadowedOptions struct {
	Header string `opt:"header" doc:"the banner every file opens with"`
	quiet  bool
}

// collidingOptions claims one config key twice.
type collidingOptions struct {
	First  string `opt:"header" doc:"one claimant"`
	Second string `opt:"header" doc:"the other claimant"`
}

// doublyWrongOptions has two independent findings: a field naming no
// key and a field stating no doc.
type doublyWrongOptions struct {
	First  string `doc:"a field naming no key"`
	Second string `                            opt:"second"`
}

// nestedOptions nests a struct that has an unexported field.
type nestedOptions struct {
	Inner innerOptions `opt:"inner" doc:"a nested option group"`
}

// innerOptions hides one field below the top level.
type innerOptions struct {
	Depth int
	quiet bool
}

// taggedOutOptions hides one field through its json tag.
type taggedOutOptions struct {
	Header string `opt:"header" doc:"the banner every file opens with"`
	Strict bool   `opt:"strict" doc:"whether the walk refuses a gap"   json:"-"`
}

// hookOptions has a field of a type the encoder refuses.
type hookOptions struct {
	Hook func() `opt:"hook" doc:"what runs after the walk"`
}

// stampedOptions has a field whose type has unexported fields and
// marshals itself.
type stampedOptions struct {
	Since time.Time `opt:"since" doc:"when the walk starts"`
}

// The options checks are what both consumers run: the composition
// before populating and fingerprinting, and the conformance suite as a
// check. Validation returns every finding at once, and the encoding is
// the one a unit key and the composition fingerprint fold, so every
// option has to be visible to it.
func TestOptions(t *testing.T) {
	t.Parallel()

	t.Run("ValidateOptions", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a plugin without the surface", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, plugin.ValidateOptions(named{name: "bare"}), "no declaration is no finding")
		})

		t.Run("returns nothing for a nil declaration", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, plugin.ValidateOptions(optioned{name: "cfg"}),
				"a nil options value reads as no options")
		})

		t.Run("returns nothing for a struct that keeps the contract", func(t *testing.T) {
			t.Parallel()

			p := optioned{name: "cfg", cfg: &lawfulOptions{Depth: 2}}
			assert.Empty(t, plugin.ValidateOptions(p),
				"exported, keyed and documented fields are the whole contract")
		})

		notPointers := []struct {
			name string
			give any
		}{
			{name: "returns an error for a struct by value", give: lawfulOptions{}},
			{name: "returns an error for a pointer to no struct", give: new(int)},
		}
		for _, tt := range notPointers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				errs := plugin.ValidateOptions(optioned{name: "cfg", cfg: tt.give})
				assert.Length(t, errs, 1, "the value is one finding")
				assert.Contains(t, errs[0].Error(), "pointer to a struct", "the finding states the contract")
				assert.Contains(t, errs[0].Error(), "cfg", "the finding names the plugin")
			})
		}

		t.Run("returns an error for a nil pointer", func(t *testing.T) {
			t.Parallel()

			errs := plugin.ValidateOptions(optioned{name: "cfg", cfg: (*lawfulOptions)(nil)})
			assert.Length(t, errs, 1, "a nil pointer is one finding")
			assert.Contains(t, errs[0].Error(), "nil", "the finding says what was returned")
		})

		t.Run("returns an error naming an unexported field", func(t *testing.T) {
			t.Parallel()

			errs := plugin.ValidateOptions(optioned{name: "cfg", cfg: &shadowedOptions{quiet: true}})
			assert.Length(t, errs, 1, "the hidden field is one finding")
			assert.Contains(t, errs[0].Error(), "quiet", "the finding names the field")
		})

		t.Run("returns an error for each field that misses a tag", func(t *testing.T) {
			t.Parallel()

			errs := plugin.ValidateOptions(optioned{name: "cfg", cfg: &doublyWrongOptions{}})
			assert.Length(t, errs, 2, "every finding arrives, not just the first")
			assert.Contains(t, errs[0].Error(), "First", "the unkeyed field is named")
			assert.Contains(t, errs[0].Error(), "opt", "the finding states the missing tag")
			assert.Contains(t, errs[1].Error(), "Second", "the undocumented field is named")
			assert.Contains(t, errs[1].Error(), "doc", "the finding states the missing tag")
		})

		t.Run("returns an error naming both fields that claim one key", func(t *testing.T) {
			t.Parallel()

			errs := plugin.ValidateOptions(optioned{name: "cfg", cfg: &collidingOptions{}})
			assert.Length(t, errs, 1, "the collision is one finding")
			assert.Contains(t, errs[0].Error(), "First", "naming one claimant")
			assert.Contains(t, errs[0].Error(), "Second", "and the other")
		})
	})

	t.Run("EncodeOptions", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a plugin without the surface", func(t *testing.T) {
			t.Parallel()

			encoded, err := plugin.EncodeOptions(named{name: "bare"})
			assert.NoError(t, err, "no declaration is no fault")
			assert.Nil(t, encoded, "and no bytes")
		})

		t.Run("returns the JSON of a visible struct", func(t *testing.T) {
			t.Parallel()

			encoded, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &lawfulOptions{Header: "h", Depth: 2}})
			assert.NoError(t, err, "every field is visible")
			assert.Equal(t, string(encoded), `{"Header":"h","Depth":2}`, "in its canonical encoding")
		})

		t.Run("returns an error naming an unexported field below the top level", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &nestedOptions{Inner: innerOptions{quiet: true}}})
			assert.HasError(t, err, "a nested field the encoding drops is refused")
			assert.Contains(t, err.Error(), "quiet", "naming the hidden field")
		})

		t.Run("returns an error naming a field its json tag hides", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &taggedOutOptions{}})
			assert.HasError(t, err, "a field the tag hides is refused")
			assert.Contains(t, err.Error(), "Strict", "naming it")
		})

		t.Run("returns the encoder's error", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &hookOptions{Hook: func() {}}})
			assert.HasError(t, err, "a function has no encoding")
			assert.Contains(t, err.Error(), "cfg", "naming the plugin")
		})

		t.Run("encodes a type that spells its own encoding", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &stampedOptions{}})
			assert.NoError(t, err, "a type that marshals itself encodes its unexported fields itself")
		})
	})
}

// The options checks allocate the reflection of the options and the
// sets they keep, and nothing for a plugin without the surface, in the
// ordinary run, which runs no benchmark. The plugins are interface
// values made once, so no check counts their conversion. The check runs
// alone, because the count includes every goroutine's allocations.
func TestOptionsAllocs(t *testing.T) {
	lawful, bare := optionPlugins()
	var errs []error
	assert.MaxAllocs(t, func() { errs = plugin.ValidateOptions(lawful) }, validateOptionsAllocs,
		"ValidateOptions allocates the claimed keys and the reflection of the fields")
	assert.Empty(t, errs, "ValidateOptions finds nothing in the lawful options")
	assert.MaxAllocs(t, func() { errs = plugin.ValidateOptions(bare) }, 0,
		"ValidateOptions allocates nothing for a plugin without the surface")

	var encoded []byte
	assert.MaxAllocs(t, func() { encoded, _ = plugin.EncodeOptions(lawful) }, encodeOptionsAllocs,
		"EncodeOptions allocates the seen types, the reflection and the bytes")
	assert.Equal(t, string(encoded), `{"Header":"h","Depth":2}`, "EncodeOptions returns the canonical encoding")
	assert.MaxAllocs(t, func() { encoded, _ = plugin.EncodeOptions(bare) }, 0,
		"EncodeOptions allocates nothing for a plugin without the surface")
}

// BenchmarkOptions measures the options checks a composition makes once
// per plugin, over the lawful options and a plugin without the surface.
func BenchmarkOptions(b *testing.B) {
	lawful, bare := optionPlugins()

	validations := []struct {
		name   string
		give   plugin.Plugin
		allocs uint64
	}{
		{name: "a struct that keeps the contract", give: lawful, allocs: validateOptionsAllocs},
		{name: "a plugin without the surface", give: bare, allocs: 0},
	}
	b.Run("ValidateOptions", func(b *testing.B) {
		for _, tt := range validations {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var errs []error
				for c.Loop() {
					errs = plugin.ValidateOptions(tt.give)
				}
				assert.Empty(b, errs, "ValidateOptions finds nothing")
			})
		}
	})

	encodings := []struct {
		name   string
		give   plugin.Plugin
		allocs uint64
	}{
		{name: "a visible struct", give: lawful, allocs: encodeOptionsAllocs},
		{name: "a plugin without the surface", give: bare, allocs: 0},
	}
	b.Run("EncodeOptions", func(b *testing.B) {
		for _, tt := range encodings {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
				defer c.End()
				var err error
				for c.Loop() {
					_, err = plugin.EncodeOptions(tt.give)
				}
				assert.NoError(b, err, "the options encode")
			})
		}
	})
}

// optionPlugins returns a plugin with the lawful options and a plugin
// without the options surface, each an interface value made once.
func optionPlugins() (lawful, bare plugin.Plugin) {
	return optioned{name: "cfg", cfg: &lawfulOptions{Header: "h", Depth: 2}}, named{name: "bare"}
}
