// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// optioned is a fixture plugin returning whatever options value it
// was built with.
type optioned struct {
	named
	cfg any
}

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

// doublyWrongOptions carries two independent findings: a field
// naming no key and a field stating no doc.
type doublyWrongOptions struct {
	First  string `doc:"a field naming no key"`
	Second string `                            opt:"second"`
}

// ValidateOptions is the one check both consumers run: the
// composition before populating, and the conformance suite as a
// check. It holds the declaration to the tag contract and returns
// every finding, so a plugin author reads every fault at once.
func TestValidateOptions(t *testing.T) {
	t.Parallel()

	t.Run("a plugin without the surface declares nothing", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, plugin.ValidateOptions(named{name: "bare"}),
			"no declaration is no finding")
	})

	t.Run("a nil declaration declares nothing", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, plugin.ValidateOptions(optioned{name: "cfg"}),
			"a nil options value reads as no options")
	})

	t.Run("a valid struct passes whole", func(t *testing.T) {
		t.Parallel()

		p := optioned{name: "cfg", cfg: &lawfulOptions{Depth: 2}}
		assert.Empty(t, plugin.ValidateOptions(p),
			"exported, keyed and documented fields are the whole contract")
	})

	t.Run("a value that is no pointer to a struct is refused", func(t *testing.T) {
		t.Parallel()

		for name, cfg := range map[string]any{
			"a struct by value":      lawfulOptions{},
			"a pointer to no struct": new(int),
		} {
			errs := plugin.ValidateOptions(optioned{name: "cfg", cfg: cfg})
			assert.Length(t, errs, 1, name+" is one finding")
			assert.Contains(t, errs[0].Error(), "pointer to a struct",
				"the finding states the contract")
			assert.Contains(t, errs[0].Error(), "cfg",
				"the finding names the plugin")
		}
	})

	t.Run("a nil pointer is refused", func(t *testing.T) {
		t.Parallel()

		p := optioned{name: "cfg", cfg: (*lawfulOptions)(nil)}
		errs := plugin.ValidateOptions(p)
		assert.Length(t, errs, 1, "a nil pointer is one finding")
		assert.Contains(t, errs[0].Error(), "nil",
			"the finding says what was returned")
	})

	t.Run("an unexported field is refused", func(t *testing.T) {
		t.Parallel()

		p := optioned{name: "cfg", cfg: &shadowedOptions{quiet: true}}
		errs := plugin.ValidateOptions(p)
		assert.Length(t, errs, 1, "the hidden field is one finding")
		assert.Contains(t, errs[0].Error(), "quiet",
			"the finding names the field")
	})

	t.Run("a field naming no opt key is refused", func(t *testing.T) {
		t.Parallel()

		p := optioned{name: "cfg", cfg: &doublyWrongOptions{}}
		errs := plugin.ValidateOptions(p)
		assert.Length(t, errs, 2, "every finding arrives, not just the first")
		assert.Contains(t, errs[0].Error(), "First",
			"the unkeyed field is named")
		assert.Contains(t, errs[0].Error(), "opt",
			"the finding states the missing tag")
		assert.Contains(t, errs[1].Error(), "Second",
			"the undocumented field is named")
		assert.Contains(t, errs[1].Error(), "doc",
			"the finding states the missing tag")
	})

	t.Run("one key claimed by two fields is refused naming both", func(t *testing.T) {
		t.Parallel()

		p := optioned{name: "cfg", cfg: &collidingOptions{}}
		errs := plugin.ValidateOptions(p)
		assert.Length(t, errs, 1, "the collision is one finding")
		assert.Contains(t, errs[0].Error(), "First", "naming one claimant")
		assert.Contains(t, errs[0].Error(), "Second", "and the other")
	})
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

// EncodeOptions is the one encoding a unit key and the composition
// fingerprint fold, so every option has to be visible to it.
func TestEncodeOptions(t *testing.T) {
	t.Parallel()

	t.Run("a plugin without the surface encodes to nothing", func(t *testing.T) {
		t.Parallel()

		encoded, err := plugin.EncodeOptions(named{name: "bare"})
		assert.NoError(t, err, "no declaration is no fault")
		assert.Nil(t, encoded, "and no bytes")
	})

	t.Run("a visible struct encodes as JSON", func(t *testing.T) {
		t.Parallel()

		encoded, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &lawfulOptions{Header: "h", Depth: 2}})
		assert.NoError(t, err, "every field is visible")
		assert.Equal(t, string(encoded), `{"Header":"h","Depth":2}`, "in its canonical encoding")
	})

	t.Run("refuses an unexported field below the top level", func(t *testing.T) {
		t.Parallel()

		_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &nestedOptions{Inner: innerOptions{quiet: true}}})
		assert.HasError(t, err, "a nested field the encoding drops is refused")
		assert.Contains(t, err.Error(), "quiet", "naming the hidden field")
	})

	t.Run("refuses a field its json tag hides", func(t *testing.T) {
		t.Parallel()

		_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &taggedOutOptions{}})
		assert.HasError(t, err, "a field the tag hides is refused")
		assert.Contains(t, err.Error(), "Strict", "naming it")
	})

	t.Run("returns the encoder's refusal", func(t *testing.T) {
		t.Parallel()

		_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &hookOptions{Hook: func() {}}})
		assert.HasError(t, err, "a function has no encoding")
		assert.Contains(t, err.Error(), "cfg", "naming the plugin")
	})

	t.Run("takes a type that spells its own encoding as it encodes", func(t *testing.T) {
		t.Parallel()

		_, err := plugin.EncodeOptions(optioned{name: "cfg", cfg: &stampedOptions{}})
		assert.NoError(t, err, "a type that marshals itself encodes its unexported fields itself")
	})
}
