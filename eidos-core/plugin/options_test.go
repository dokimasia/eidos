// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// optioned is a fixture plugin answering whatever options value it
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
// check. It holds the declaration to the tag contract and answers
// every finding, so a plugin author reads the whole bill at once.
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

	t.Run("a lawful struct passes whole", func(t *testing.T) {
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
			"the finding says what was answered")
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
		assert.Length(t, errs, 2, "every finding lands, not just the first")
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
