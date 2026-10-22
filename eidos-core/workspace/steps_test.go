// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The values with which the refinement cases refine a plan, an output
// directory, an import base, a pattern and a filename.
const (
	refinedDir        = "gen"
	refinedImportBase = "example.com/platform/gen"
	refinedPattern    = "svc/..."
	refinedFile       = "all.txt"
)

// The binding fixture beside [flaggerID], the annotator that registers
// and stamps the flag key. auditID is the annotator that gates on the
// key through a named handle, flagsCap is the capability that orders
// the two, flagKeyName is the key, and ghostFlagName is a name that
// nothing registers. absentKeyName is a name in a namespace that nothing
// claims, and absentCap is a capability that nothing provides.
const (
	auditID       plugin.ID         = "audit"
	flagsCap      plugin.Capability = "flags"
	flagKeyName   meta.KeyName      = "flag.marked"
	ghostFlagName meta.KeyName      = "flag.ghost"
	absentKeyName meta.KeyName      = "absent.marked"
	absentCap     plugin.Capability = "absent"
)

// The dual fixture of the ordering cases. duoID is the plugin with both
// roles, makerID is the generator that provides the capability that duoID
// requires or runs after, and duoKeyName is the key that the annotator half
// of duoID stamps. The name duoID sorts before makerID, so only the
// capability orders the two generators.
const (
	duoID      plugin.ID    = "duo"
	makerID    plugin.ID    = "maker"
	duoKeyName meta.KeyName = "duo.seen"
)

// The language of [keyedFrontend] and the key that it registers.
const (
	keyedLang symbol.Lang  = "kv"
	keyedKey  meta.KeyName = "kv.flag"
)

// The config texts of [pacedOptions]. timeoutText is a duration, lateText
// is not a duration, and slowText is not a pace.
const (
	timeoutText = "1m30s"
	lateText    = "soon"
	slowText    = "slow"
)

// errPace is the error of [pacedMode.UnmarshalText] for a text that is not
// a pace.
var errPace = errors.New("the text is not a pace")

// keyedFrontend is the scripted frontend under another language, which
// registers the keys of its language through register.
type keyedFrontend struct {
	*frontendtest.Scripted
	lang     symbol.Lang
	register func(*meta.Registry) error
}

// Lang returns the declared language.
func (f keyedFrontend) Lang() symbol.Lang { return f.lang }

// Keys registers the language's keys through the declared function.
func (f keyedFrontend) Keys(r *meta.Registry) error { return f.register(r) }

// convertedBound is a struct that an option of [convertedOptions] has.
type convertedBound struct {
	Files int `json:"files"`
}

// convertedOptions are options whose fields have types that a decoder of a
// config file does not return.
type convertedOptions struct {
	Limit int64          `opt:"limit" doc:"the largest number of files"`
	Tags  []string       `opt:"tags"  doc:"the tags of each file"`
	Bound convertedBound `opt:"bound" doc:"the bounds of a file"`
}

// pacedMode is a closed set of paces, as Go states an enumeration. Its
// UnmarshalText refuses a text outside the set.
type pacedMode string

// The paces of [pacedMode].
const (
	paceFast pacedMode = "fast"
	paceSafe pacedMode = "safe"
)

// UnmarshalText sets m to the pace in text. It returns [errPace] for a text
// that is not a pace.
func (m *pacedMode) UnmarshalText(text []byte) error {
	pace := pacedMode(text)
	if pace != paceFast && pace != paceSafe {
		return errPace
	}
	*m = pace
	return nil
}

// pacedOptions are options that take their values from text. Timeout takes
// a duration and Mode a pace.
type pacedOptions struct {
	Timeout time.Duration `opt:"timeout" doc:"how long the walk waits"`
	Mode    pacedMode     `opt:"mode"    doc:"how fast the walk runs"`
}

// requiredOptions has a required banner.
type requiredOptions struct {
	Header string `opt:"header" doc:"the banner every file opens with" schema:"required"`
}

// nameless is a hand-rolled annotator that returns no name. The facade
// cannot build one. The roster refuses it, because findings, units and
// claims all use the name as their key.
type nameless struct{}

// Name returns the empty name.
func (nameless) Name() plugin.ID { return "" }

// Annotate stamps nothing.
func (nameless) Annotate(*plugin.AnnotatorContext) error { return nil }

// handmade is a hand-rolled annotator that a case passes as a struct
// value with a slice field, so its type is not comparable. The facade
// builds pointers. The composition reads a plugin of any shape that the
// service provider interface admits.
type handmade struct {
	name   plugin.ID
	labels []string
}

// Name returns the declared name.
func (h handmade) Name() plugin.ID { return h.name }

// Annotate stamps nothing.
func (handmade) Annotate(*plugin.AnnotatorContext) error { return nil }

// declaring is a hand-rolled annotator that returns its capability lists
// verbatim, with duplicates and empty labels. The facade refuses an empty
// label at its own typed build. The composition checks a label that
// arrives through the service provider interface.
type declaring struct {
	name     plugin.ID
	provides []plugin.Capability
	requires []plugin.Capability
	after    []plugin.Capability
}

// Name returns the declared name.
func (d *declaring) Name() plugin.ID { return d.name }

// Annotate stamps nothing.
func (*declaring) Annotate(*plugin.AnnotatorContext) error { return nil }

// Priority places every role at priority one.
func (*declaring) Priority(plugin.Role) int { return 1 }

// Provides returns the provided labels verbatim.
func (d *declaring) Provides() []plugin.Capability { return d.provides }

// Requires returns the required labels verbatim.
func (d *declaring) Requires() []plugin.Capability { return d.requires }

// After returns the labels that the annotator runs after, verbatim.
func (d *declaring) After() []plugin.Capability { return d.after }

// partial is a hand-rolled annotator that implements Priority and Provides
// of [plugin.CapabilityProvider], and lacks After and Requires.
type partial struct{}

// Name returns the name partial.
func (partial) Name() plugin.ID { return "partial" }

// Annotate stamps nothing.
func (partial) Annotate(*plugin.AnnotatorContext) error { return nil }

// Priority places every role at priority one.
func (partial) Priority(plugin.Role) int { return 1 }

// Provides returns no label.
func (partial) Provides() []plugin.Capability { return nil }

// treeless is a hand-rolled generator that implements Templates of
// [plugin.TemplateProvider], and lacks its three other methods.
type treeless struct{}

// Name returns the name treeless.
func (treeless) Name() plugin.ID { return "treeless" }

// Generate emits nothing.
func (treeless) Generate(*plugin.GeneratorContext) error { return nil }

// Templates returns no tree.
func (treeless) Templates(plugin.Target) (fs.FS, bool) { return nil, false }

// misspelled is a hand-rolled annotator whose After has another signature
// than the method of [plugin.CapabilityProvider].
type misspelled struct{ partial }

// Name returns the name misspelled.
func (misspelled) Name() plugin.ID { return "misspelled" }

// Requires returns no label.
func (misspelled) Requires() []plugin.Capability { return nil }

// After returns no label, as plain strings.
func (misspelled) After() []string { return nil }

// rebinding is a hand-rolled annotator whose BindKeys returns a plugin of
// another type.
type rebinding struct{}

// Name returns the name rebinding.
func (rebinding) Name() plugin.ID { return "rebinding" }

// Annotate stamps nothing.
func (rebinding) Annotate(*plugin.AnnotatorContext) error { return nil }

// BindKeys returns a hand-made annotator of the same name.
func (rebinding) BindKeys(*meta.Registry) (plugin.Plugin, error) {
	return handmade{name: "rebinding"}, nil
}

// unclaiming is a hand-rolled annotator without capabilities whose bind
// refuses a name of a namespace that nothing claims.
type unclaiming struct{}

// Name returns the name unclaiming.
func (unclaiming) Name() plugin.ID { return "unclaiming" }

// Annotate stamps nothing.
func (unclaiming) Annotate(*plugin.AnnotatorContext) error { return nil }

// BindKeys returns u and the error of a name of a namespace that nothing
// claims.
func (u unclaiming) BindKeys(*meta.Registry) (plugin.Plugin, error) {
	return u, plugin.ErrUnclaimedNamespace
}

// TestSteps tests the steps of Build, whose output is the schedule. A
// case observes the schedule in the order in which the handlers run,
// and in the turn that every claim records.
func TestSteps(t *testing.T) {
	t.Parallel()

	t.Run("assemble", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming a plugin without a name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(nameless{}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "empty name", "the error names the fault")
		})

		t.Run("returns no error for a plugin whose type is not comparable", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(handmade{name: "handmade", labels: []string{"a"}}).Build()
			assert.NoError(t, err, "the composition reads the plugin through its name")
		})

		t.Run("returns no error for one provider listed twice", func(t *testing.T) {
			t.Parallel()

			one := &handmade{name: "listed"}
			_, err := valid().Annotators(one, one).Build()
			assert.NoError(t, err, "the repeated pointer is the same provider")
		})

		t.Run("returns an error naming two providers under one name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(&handmade{name: "twinned"}, &handmade{name: "twinned"}).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "twinned", "the error names the contended name")
		})
	})

	t.Run("register", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin's key registration error", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(keyed("keyless", func(*meta.Registry) error {
					return errors.New("the shape namespace is taken")
				})).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "namespace is taken", "the error has the provider's cause")
		})

		t.Run("returns an error naming a plugin's key in another plugin's namespace", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(
					keyed("shape", func(r *meta.Registry) error { return r.ClaimNamespace("shape") }),
					keyed("rival", func(r *meta.Registry) error {
						_, err := meta.Register[string](r, meta.KeySpec{
							Name: "shape.role", Doc: "a key under a foreign namespace",
						})
						return err
					}),
				).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `"rival"`, "the error names the registering plugin")
			assert.Contains(t, err.Error(), `"shape"`, "the error names the claiming plugin")
		})

		t.Run("returns a plugin's schema registration error", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(planTo("second", "fixture",
				schemad("picky", directive.Schema{Plugin: "picky", Name: "gate"}))).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "states no semantics", "the error names the fault")
			assert.Contains(t, err.Error(), "gate", "the error names the schema")
		})

		t.Run("returns an error naming a plugin's schema under another plugin's name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(planTo("second", "fixture",
				schemad("impostor", directive.Schema{Plugin: "picky", Name: "gate", Doc: "a borrowed name"}))).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "impostor", "the error names the declaring plugin")
			assert.Contains(t, err.Error(), `"picky"`, "the error names the borrowed name")
		})

		t.Run("registers a frontend's keys under its language's spelling", func(t *testing.T) {
			t.Parallel()

			w := built(t, valid().Frontends(keyedFrontend{
				Scripted: frontendtest.NewScripted(), lang: keyedLang, register: keyedKeys,
			}))
			target, err := w.ParseTarget(string(keyedKey) + "@svc/store/row.zz:1")
			assert.NoError(t, err, "the frontend's key is registered")
			assert.Equal(t, target.Key, keyedKey, "the target has the frontend's key")
		})

		t.Run("returns a frontend's key registration error", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Frontends(keyedFrontend{
				Scripted: frontendtest.NewScripted(), lang: keyedLang,
				register: func(*meta.Registry) error { return errors.New("the kv namespace is taken") },
			}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "the kv namespace is taken", "the error has the frontend's cause")
		})

		t.Run("returns an error for the composition's claim of a frontend's namespace", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Frontends(keyedFrontend{Scripted: frontendtest.NewScripted(), lang: keyedLang, register: keyedKeys}).
				Keys(func(r *meta.Registry) error { return r.ClaimNamespace(string(keyedLang)) }).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `namespace "kv" is claimed twice: by "kv" and by the composition`,
				"the error contains the language and the composition")
		})

		sealedRegistry := func(t *testing.T) *meta.Registry {
			t.Helper()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition builds")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the composition runs")
			return report.Facts.Registry()
		}

		t.Run("seals the key registry against a namespace claim", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, sealedRegistry(t).ClaimNamespace("late"), "the claim fails")
		})

		t.Run("seals the key registry against a key registration", func(t *testing.T) {
			t.Parallel()

			_, err := meta.Register[string](sealedRegistry(t), meta.KeySpec{
				Name: "late.role", Doc: "registered after the seal",
			})
			assert.HasError(t, err, "the registration fails")
		})
	})

	t.Run("capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming a plugin that provides an empty label", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("hollow", caps(""), nil)).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "provides an empty capability label", "the error names the side")
			assert.Contains(t, err.Error(), "hollow", "the error names the plugin")
		})

		t.Run("returns an error naming a plugin that requires an empty label", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("wanting", nil, caps(""))).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "requires an empty capability label", "the error names the side")
			assert.Contains(t, err.Error(), "wanting", "the error names the plugin")
		})

		t.Run("counts a label one plugin provides twice as one provider", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("twice", caps("json", "json"), nil)).Build()
			assert.NoError(t, err, "the label does not collide with itself")
		})

		t.Run("reports a label one plugin requires twice once", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("asking", nil, caps("missing", "missing"))).Build()
			assert.HasError(t, err, "nothing provides the label")
			assert.Equal(t, strings.Count(err.Error(), `requires capability "missing"`), 1,
				"the gap is reported once")
		})

		t.Run("returns an error naming a plugin that runs after an empty label", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(&declaring{name: "trailing", after: caps("")}).Build()
			assert.HasError(t, err, "the composition fails")
			expect.That(t, err.Error()).
				Contains("runs after an empty capability label", "the error names the side").
				Contains("trailing", "the error names the plugin")
		})

		t.Run("returns no error for a label that a plugin runs after and nothing provides", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(&declaring{name: "trailing", after: caps("absent")}).Build()
			assert.NoError(t, err, "a label that the plugin runs after needs no provider")
		})
	})

	t.Run("partialProviders", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			compose func() *workspace.Builder
			want    string
		}{
			{
				name:    "returns an error naming the methods that a partial capability provider lacks",
				compose: func() *workspace.Builder { return valid().Annotators(partial{}) },
				want:    "partial implements Priority and Provides of plugin.CapabilityProvider, and lacks After and Requires",
			},
			{
				name: "returns an error naming the methods that a partial template provider lacks",
				compose: func() *workspace.Builder {
					return valid().Plans(planTo("second", "fixture", treeless{}))
				},
				want: "treeless implements Templates of plugin.TemplateProvider, " +
					"and lacks Overrides and TemplateFuncs and TemplateTargets",
			},
			{
				name:    "returns an error naming a method of another signature as lacking",
				compose: func() *workspace.Builder { return valid().Annotators(misspelled{}) },
				want: "misspelled implements Priority and Provides and Requires of plugin.CapabilityProvider, " +
					"and lacks After",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := tt.compose().Build()
				assert.HasError(t, err, "the composition fails")
				assert.Contains(t, err.Error(), tt.want, "the error names what the plugin implements and lacks")
			})
		}
	})

	t.Run("configure", func(t *testing.T) {
		t.Parallel()

		t.Run("replaces a constructed default with the configured value", func(t *testing.T) {
			t.Parallel()

			opts := &mirrorOptions{Depth: 1}
			w, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.NoError(t, err, "the section meets the tag contract")
			assert.NotNil(t, w, "the workspace composes")
			assert.Equal(t, opts.Depth, 3, "the plugin's struct has the configured value")
		})

		t.Run("converts a value of another type through the JSON encoding", func(t *testing.T) {
			t.Parallel()

			opts := &convertedOptions{}
			_, err := sectioned("tuned", opts, map[string]any{
				"limit": 3, "tags": []any{"api", "store"}, "bound": map[string]any{"files": 2},
			}).Build()
			assert.NoError(t, err, "each value decodes into its field")
			expect.Equal(t, opts.Limit, int64(3), "an int fills an int64 field")
			expect.Equal(t, opts.Tags, []string{"api", "store"}, "a list of strings fills a []string field")
			expect.Equal(t, opts.Bound, convertedBound{Files: 2}, "a mapping fills a struct field")
		})

		t.Run("parses a text into a time.Duration field", func(t *testing.T) {
			t.Parallel()

			opts := &pacedOptions{}
			_, err := sectioned("tuned", opts, map[string]any{"timeout": timeoutText}).Build()
			assert.NoError(t, err, "the text parses as a duration")
			want, err := time.ParseDuration(timeoutText)
			assert.NoError(t, err, "the fixture text is a duration")
			assert.Equal(t, opts.Timeout, want, "the field has the duration of the text")
		})

		t.Run("decodes a number of nanoseconds into a time.Duration field", func(t *testing.T) {
			t.Parallel()

			opts := &pacedOptions{}
			_, err := sectioned("tuned", opts, map[string]any{"timeout": 1500}).Build()
			assert.NoError(t, err, "the number decodes into the duration")
			assert.Equal(t, opts.Timeout, 1500*time.Nanosecond, "the field has the number as nanoseconds")
		})

		t.Run("decodes a text through the UnmarshalText of the type of the field", func(t *testing.T) {
			t.Parallel()

			opts := &pacedOptions{}
			_, err := sectioned("tuned", opts, map[string]any{"mode": string(paceSafe)}).Build()
			assert.NoError(t, err, "the type takes the text")
			assert.Equal(t, opts.Mode, paceSafe, "the field has the decoded pace")
		})

		t.Run("returns the wrapped error of ParseDuration for a text that is not a duration", func(t *testing.T) {
			t.Parallel()

			_, want := time.ParseDuration(lateText)
			assert.HasError(t, want, "the fixture text is not a duration")
			_, err := sectioned("tuned", &pacedOptions{}, map[string]any{"timeout": lateText}).Build()
			assert.HasError(t, err, "the text does not parse")
			assert.Contains(t, err.Error(), `option "timeout" of plugin "tuned" takes time.Duration: `+want.Error(),
				"the message contains the option and the error of ParseDuration")
		})

		t.Run("returns the wrapped error of the decoder for a text that the type refuses", func(t *testing.T) {
			t.Parallel()

			_, err := sectioned("tuned", &pacedOptions{}, map[string]any{"mode": slowText}).Build()
			assert.ErrorIs(t, err, errPace, "the error wraps the refusal of UnmarshalText")
			assert.Contains(t, err.Error(), `option "mode" of plugin "tuned" takes workspace_test.pacedMode: `,
				"the message contains the option and its type")
		})

		t.Run("returns the wrapped error of the encoder for a value that does not encode", func(t *testing.T) {
			t.Parallel()

			_, err := sectioned("tuned", &mirrorOptions{}, map[string]any{"depth": func() {}}).Build()
			unsupported := assert.ErrorAs[*json.UnsupportedTypeError](t, err,
				"the error wraps the error of the encoder")
			assert.Equal(t, unsupported.Type, reflect.TypeFor[func()](), "the encoder refuses the type of the value")
			assert.Contains(t, err.Error(), `option "depth" of plugin "tuned" takes int: `,
				"the message contains the option and its type")
		})

		t.Run("returns the wrapped error of the decoder for a value of another shape", func(t *testing.T) {
			t.Parallel()

			_, err := sectioned("tuned", &mirrorOptions{}, map[string]any{"depth": map[string]any{"deep": 1}}).Build()
			mismatch := assert.ErrorAs[*json.UnmarshalTypeError](t, err, "the error wraps the error of the decoder")
			assert.Equal(t, mismatch.Type, reflect.TypeFor[int](), "the decoder reports the type of the field")
			assert.Contains(t, err.Error(), `option "depth" of plugin "tuned" takes int: `,
				"the message contains the option and its type")
		})

		t.Run("returns an error for an option without a value", func(t *testing.T) {
			t.Parallel()

			_, err := sectioned("tuned", &mirrorOptions{}, map[string]any{"depth": nil}).Build()
			assert.HasError(t, err, "the option has no value of its type")
			assert.Contains(t, err.Error(), `option "depth" of plugin "tuned" takes int, and the config has nothing`,
				"the message contains the option and its type")
		})

		t.Run("returns an error naming a mapping with a key that the struct does not declare", func(t *testing.T) {
			t.Parallel()

			_, err := sectioned("tuned", &convertedOptions{}, map[string]any{
				"bound": map[string]any{"filez": 2},
			}).Build()
			assert.HasError(t, err, "the decoder rejects the unknown key")
			assert.Contains(t, err.Error(), `"bound"`, "the error names the option")
		})

		t.Run("skips populating options whose declaration fails", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Depth int `opt:"depth"`
			}{}
			_, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.HasError(t, err, "the options struct is undocumented")
			assert.Contains(t, err.Error(), "doc", "the error names the fault")
			assert.Equal(t, opts.Depth, 0, "the struct keeps its constructed value")
		})

		t.Run("returns an error naming an options field the encoding hides", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Depth  int  `opt:"depth"  doc:"how deep the mirror walks"`
				Strict bool `opt:"strict" doc:"whether the walk refuses a gap" json:"-"`
			}{}
			_, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.HasError(t, err, "the fingerprint folds every option")
			assert.Contains(t, err.Error(), "Strict", "the error names the hidden field")
		})

		t.Run("returns an error naming a plugin whose options the encoding rejects", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Hook func() `opt:"hook" doc:"what runs after the walk"`
			}{}
			_, err := valid().Plans(planTo("second", "fixture", tuned("tuned", opts))).Build()
			assert.HasError(t, err, "options without an encoding have no fingerprint")
			assert.Contains(t, err.Error(), "tuned", "the error names the plugin")
		})

		t.Run("returns an error for a section of a plugin without options", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Config(workspace.Config{
				Options: map[string]map[string]any{"mirror": {"depth": 1}},
			}).Build()
			assert.HasError(t, err, "a section nothing reads is a typo")
			assert.Contains(t, err.Error(), "declares no options", "the error names the fault")
			assert.Contains(t, err.Error(), `"mirror"`, "the error names the plugin")
		})

		t.Run("returns an error naming a required option with the zero value", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(planTo("second", "fixture", tuned("tuned", &requiredOptions{}))).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "plugin: tuned requires the option header",
				"the message contains the plugin and the key of the option")
		})

		t.Run("returns no error for a required option that the config sets", func(t *testing.T) {
			t.Parallel()

			opts := &requiredOptions{}
			_, err := sectioned("tuned", opts, map[string]any{"header": "// banner"}).Build()
			assert.NoError(t, err, "the config gives the option a value")
			assert.Equal(t, opts.Header, "// banner", "the field has the configured value")
		})

		t.Run("returns no error for a required option that the constructor sets", func(t *testing.T) {
			t.Parallel()

			opts := &requiredOptions{Header: "// banner"}
			_, err := valid().Plans(planTo("second", "fixture", tuned("tuned", opts))).Build()
			assert.NoError(t, err, "the constructed value counts as a value")
		})
	})

	t.Run("refine", func(t *testing.T) {
		t.Parallel()

		dependent := func() *workspace.Builder {
			bindings := planTo("bindings", "fixture", mirror("bindings-mirror"))
			bindings.DependsOn = []string{"plan"}
			return valid().Plans(bindings).
				Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {Disabled: true}}})
		}
		read := func() *workspace.Builder {
			return valid().Checks(&recordingCheck{name: "stubbed", reads: []string{"plan"}}).
				Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {Disabled: true}}})
		}
		closed := func(r workspace.PlanConfig) func() *workspace.Builder {
			return func() *workspace.Builder {
				return valid().Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": r}})
			}
		}
		extensible := func() *workspace.Builder {
			pl := planTo("plan", "fixture", mirror("mirror"))
			pl.Extensible = true
			return workspace.New().Brand(fixtureBrand).Annotators(stamper("noter", quiet)).Targets("fixture").Plans(pl)
		}
		extended := func(gens ...plugin.Generator) *workspace.Builder {
			return extensible().Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {Generators: gens}}})
		}
		disabled := func(pl workspace.Plan) func() *workspace.Builder {
			return func() *workspace.Builder {
				return valid().Plans(pl).
					Config(workspace.Config{Plans: map[string]workspace.PlanConfig{pl.Name: {Disabled: true}}})
			}
		}

		refused := []struct {
			name    string
			compose func() *workspace.Builder
			want    string
		}{
			{
				name: "returns an error naming a plan that the composition does not declare",
				compose: func() *workspace.Builder {
					return valid().Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"absent": {}}})
				},
				want: `the config refines plan "absent", which the composition does not declare`,
			},
			{
				name:    "returns an error naming a disabled plan that an enabled plan depends on",
				compose: dependent,
				want:    `the config disables plan "plan", and plan "bindings" depends on it`,
			},
			{
				name:    "returns an error naming a disabled plan that a check reads",
				compose: read,
				want:    `the config disables plan "plan", and check stubbed reads it`,
			},
			{
				name:    "returns an error naming a plan that is not extensible for the generators of the config",
				compose: closed(workspace.PlanConfig{Generators: []plugin.Generator{mirror("added")}}),
				want:    `the config adds generators to plan "plan", which is not extensible`,
			},
			{
				name: "returns an error naming a disabled plan that is not extensible for the generators of the config",
				compose: closed(workspace.PlanConfig{
					Disabled: true, Generators: []plugin.Generator{mirror("added")},
				}),
				want: `the config adds generators to plan "plan", which is not extensible`,
			},
			{
				name:    "returns an error naming the plan of a nil generator of the config",
				compose: func() *workspace.Builder { return extended(nil) },
				want:    `plan "plan" lists a nil generator`,
			},
			{
				name:    "returns an error naming a disabled plan that lists a nil generator",
				compose: disabled(planTo("bindings", "fixture", nil)),
				want:    `the config disables plan "bindings", which lists a nil generator`,
			},
			{
				name: "returns an error naming a disabled plan that declares no backend",
				compose: disabled(workspace.Plan{
					Name: "bindings", Generators: []plugin.Generator{mirror("bindings-mirror")},
				}),
				want: `the config disables plan "bindings", which declares no backend`,
			},
			{
				name:    "returns an error naming a generator refinement of the config for a generator outside the plan",
				compose: closed(workspace.PlanConfig{Plugins: map[plugin.ID]layout.Refinement{"ghost": {}}}),
				want:    `plan "plan": a refinement names ghost, which is no generator of the plan`,
			},
			{
				name: "returns an error naming a family refinement of the config for a family the generator lacks",
				compose: closed(workspace.PlanConfig{Families: map[layout.Family]layout.Refinement{
					{Plugin: "mirror", Tag: "bogus"}: {},
				}}),
				want: `plan "plan": a refinement names family "bogus" of mirror, which mirror does not declare`,
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := tt.compose().Build()
				assert.HasError(t, err, "the refinement is refused")
				assert.Contains(t, err.Error(), tt.want, "the error names the plan")
			})
		}

		kept := []struct {
			name    string
			compose func() *workspace.Builder
			absent  string
		}{
			{
				name:    "keeps a disabled plan that an enabled plan depends on",
				compose: dependent,
				absent:  `depends on "plan", which the composition does not declare`,
			},
			{
				name:    "keeps a disabled plan that a check reads",
				compose: read,
				absent:  `reads plan "plan", which the composition does not declare`,
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := tt.compose().Build()
				assert.HasError(t, err, "the refinement is refused")
				assert.NotContains(t, err.Error(), tt.absent, "the steps after the refinement find the plan")
			})
		}

		t.Run("returns an error for a nil check beside a plan refinement", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Checks(nil).
				Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {}}}).
				Build()
			assert.HasError(t, err, "a nil check is refused")
			assert.Contains(t, err.Error(), "check 1 of 1 is nil", "the roster reports the nil check")
		})

		t.Run("leaves a plan that the config disables out of the workspace", func(t *testing.T) {
			t.Parallel()

			w := built(t, valid().Plans(planTo("bindings", "fixture", mirror("bindings-mirror"))).
				Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"bindings": {Disabled: true}}}))
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Plans, []workspace.PlanReport{{Name: "plan", Status: workspace.PlanCommitted}},
				"the run runs the enabled plan alone")
		})

		t.Run("leaves the declared plans as the composition states them", func(t *testing.T) {
			t.Parallel()

			b := valid().Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {Dir: refinedDir}}})
			_, err := b.Build()
			assert.NoError(t, err, "the refined composition builds")
			assert.Equal(t, built(t, b.Config(workspace.Config{})).Fingerprint(), built(t, valid()).Fingerprint(),
				"a build without the refinement has the same fingerprint as the declared composition")
		})

		t.Run("leaves the fingerprint unchanged for a refinement that sets nothing", func(t *testing.T) {
			t.Parallel()

			got := built(t, valid().Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {}}}))
			assert.Equal(t, got.Fingerprint(), built(t, valid()).Fingerprint(),
				"the refined composition has the same fingerprint as the declared one")
		})

		folded := []struct {
			name    string
			give    workspace.PlanConfig
			declare func(p *workspace.Plan)
		}{
			{
				name:    "folds a refined policy into the fingerprint as a declared one",
				give:    workspace.PlanConfig{Policy: layout.PolicyAlongside},
				declare: func(p *workspace.Plan) { p.Layout.Policy = layout.PolicyAlongside },
			},
			{
				name:    "folds a refined directory into the fingerprint as a declared one",
				give:    workspace.PlanConfig{Dir: refinedDir},
				declare: func(p *workspace.Plan) { p.Layout.Dir = refinedDir },
			},
			{
				name:    "folds a refined import base into the fingerprint as a declared one",
				give:    workspace.PlanConfig{ImportBase: refinedImportBase},
				declare: func(p *workspace.Plan) { p.Layout.ImportBase = refinedImportBase },
			},
			{
				name: "folds refined sources into the fingerprint as declared ones",
				give: workspace.PlanConfig{Sources: &workspace.Sources{Packages: []string{refinedPattern}}},
				declare: func(p *workspace.Plan) {
					p.Sources = workspace.Sources{Packages: []string{refinedPattern}}
				},
			},
			{
				name: "folds a generator refinement into the fingerprint as a declared one",
				give: workspace.PlanConfig{Plugins: map[plugin.ID]layout.Refinement{"mirror": {File: refinedFile}}},
				declare: func(p *workspace.Plan) {
					p.Layout.Plugins = map[plugin.ID]layout.Refinement{"mirror": {File: refinedFile}}
				},
			},
			{
				name: "folds a family refinement into the fingerprint as a declared one",
				give: workspace.PlanConfig{Families: map[layout.Family]layout.Refinement{
					{Plugin: "mirror"}: {File: refinedFile},
				}},
				declare: func(p *workspace.Plan) {
					p.Layout.Families = map[layout.Family]layout.Refinement{{Plugin: "mirror"}: {File: refinedFile}}
				},
			},
		}
		for _, tt := range folded {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				declared := planTo("plan", "fixture", mirror("mirror"))
				tt.declare(&declared)
				want := built(t, workspace.New().
					Brand(fixtureBrand).Annotators(stamper("noter", quiet)).Targets("fixture").Plans(declared))
				assert.NotEqual(t, want.Fingerprint(), built(t, valid()).Fingerprint(),
					"the fingerprint folds the declared field")
				refined := workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": tt.give}}
				got := built(t, valid().Config(refined))
				assert.Equal(t, got.Fingerprint(), want.Fingerprint(),
					"the refined composition has the same fingerprint as the composition that declares the field")
			})
		}

		t.Run("merges a generator refinement of the config over the declared one", func(t *testing.T) {
			t.Parallel()

			declared := map[plugin.ID]layout.Refinement{"mirror": {Policy: layout.PolicyCentralised, Dir: refinedDir}}
			pl := planTo("plan", "fixture", mirror("mirror"))
			pl.Layout.Plugins = declared
			b := workspace.New().Brand(fixtureBrand).Annotators(stamper("noter", quiet)).Targets("fixture").Plans(pl).
				Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {
					Plugins: map[plugin.ID]layout.Refinement{"mirror": {File: refinedFile}},
				}}})
			plans := built(t, b).Describe().Plans
			assert.Length(t, plans, 1, "the composition has one plan")
			assert.Equal(t, plans[0].Layout.Plugins, map[plugin.ID]layout.Refinement{
				"mirror": {Policy: layout.PolicyCentralised, Dir: refinedDir, File: refinedFile},
			}, "the config sets the filename, and the plan keeps its policy and its directory")
			assert.Equal(t, declared,
				map[plugin.ID]layout.Refinement{"mirror": {Policy: layout.PolicyCentralised, Dir: refinedDir}},
				"the refinement of the declared plan is the same as before the build")
		})

		t.Run("merges a family refinement of the config over the declared one", func(t *testing.T) {
			t.Parallel()

			family := layout.Family{Plugin: "mirror"}
			declared := map[layout.Family]layout.Refinement{family: {File: refinedFile}}
			pl := planTo("plan", "fixture", mirror("mirror"))
			pl.Layout.Families = declared
			centralised := layout.Refinement{Policy: layout.PolicyCentralised, Dir: refinedDir}
			b := workspace.New().Brand(fixtureBrand).Annotators(stamper("noter", quiet)).Targets("fixture").Plans(pl).
				Config(workspace.Config{Plans: map[string]workspace.PlanConfig{"plan": {
					Families: map[layout.Family]layout.Refinement{family: centralised},
				}}})
			plans := built(t, b).Describe().Plans
			assert.Length(t, plans, 1, "the composition has one plan")
			assert.Equal(t, plans[0].Layout.Families, map[layout.Family]layout.Refinement{
				family: {Policy: layout.PolicyCentralised, Dir: refinedDir, File: refinedFile},
			}, "the config sets the policy and the directory, and the plan keeps its filename")
			assert.Equal(t, declared, map[layout.Family]layout.Refinement{family: {File: refinedFile}},
				"the refinement of the declared plan is the same as before the build")
		})

		t.Run("appends the generators of the config to an extensible plan", func(t *testing.T) {
			t.Parallel()

			plans := built(t, extended(mirror("added"))).Describe().Plans
			assert.Length(t, plans, 1, "the composition has one plan")
			assert.Equal(t, plans[0].Generate,
				[]workspace.Component{{Name: "added", Turn: 1}, {Name: "mirror", Turn: 2}},
				"the plan runs the generator that it declares and the generator of the config")
		})

		t.Run("schedules a generator that the config adds to two plans once", func(t *testing.T) {
			t.Parallel()

			added := mirror("added")
			first := planTo("first", "fixture", mirror("first-mirror"))
			second := planTo("second", "fixture", mirror("second-mirror"))
			first.Extensible, second.Extensible = true, true
			b := workspace.New().Brand(fixtureBrand).Annotators(stamper("noter", quiet)).Targets("fixture")
			plans := built(t, b.Plans(first, second).Config(workspace.Config{Plans: map[string]workspace.PlanConfig{
				"first":  {Generators: []plugin.Generator{added}},
				"second": {Generators: []plugin.Generator{added}},
			}})).Describe().Plans
			assert.Length(t, plans, 2, "the composition has two plans")
			expect.Contains(t, plans[0].Generate, workspace.Component{Name: "added", Turn: 1},
				"the first plan runs the generator in the first turn")
			expect.Contains(t, plans[1].Generate, workspace.Component{Name: "added", Turn: 1},
				"the second plan runs the generator in the same turn as the first plan")
		})

		t.Run("populates the options of a generator of the config", func(t *testing.T) {
			t.Parallel()

			opts := &mirrorOptions{Depth: 1}
			_, err := extensible().Config(workspace.Config{
				Options: map[string]map[string]any{"tuned": {"depth": 3}},
				Plans: map[string]workspace.PlanConfig{"plan": {
					Generators: []plugin.Generator{tuned("tuned", opts)},
				}},
			}).Build()
			assert.NoError(t, err, "the composition contains the generator of the config")
			assert.Equal(t, opts.Depth, 3, "the options struct of the generator has the configured value")
		})

		t.Run("folds the generators of the config into the fingerprint as declared ones", func(t *testing.T) {
			t.Parallel()

			want := built(t, workspace.New().Brand(fixtureBrand).Annotators(stamper("noter", quiet)).Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"), mirror("added"))))
			assert.NotEqual(t, want.Fingerprint(), built(t, valid()).Fingerprint(),
				"the fingerprint folds the declared generator")
			assert.Equal(t, built(t, extended(mirror("added"))).Fingerprint(), want.Fingerprint(),
				"the extended composition has the same fingerprint as the composition that declares the generator")
		})
	})

	t.Run("lower", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the lower priority first", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			runOrdering(t,
				ordered("late", 5, nil, nil, &calls),
				ordered("early", 1, nil, nil, &calls),
			)
			assert.Equal(t, calls, []plugin.ID{"early", "late"}, "the declaration order does not decide")
		})

		t.Run("runs a provider before its requirer within one priority", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			runOrdering(t,
				ordered("needs", 1, nil, caps("cap"), &calls),
				ordered("gives", 1, caps("cap"), nil, &calls),
			)
			assert.Equal(t, calls, []plugin.ID{"gives", "needs"}, "the names do not decide")
		})

		t.Run("orders by name what priorities and capabilities leave open", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			runOrdering(t,
				ordered("beta", 1, nil, nil, &calls),
				ordered("alph", 1, nil, nil, &calls),
			)
			assert.Equal(t, calls, []plugin.ID{"alph", "beta"}, "the names break the tie")
		})

		t.Run("runs a provider before a plugin that runs after it within one priority", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			runOrdering(t,
				following("follows", 1, &calls),
				ordered("gives", 1, caps("cap"), nil, &calls),
			)
			assert.Equal(t, calls, []plugin.ID{"gives", "follows"}, "the names do not decide")
		})

		t.Run("returns an error naming a plugin that runs after a capability at a later priority", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			_, err := valid().Annotators(following("follows", 1, &calls), ordered("gives", 2, caps("cap"), nil, &calls)).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(),
				`annotator follows runs after capability "cap" at priority 1, and gives provides it at priority 2`,
				"the error names both plugins and both priorities")
		})

		t.Run("returns an error naming an annotator that runs after a capability of a generator", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			_, err := valid().
				Annotators(following("follows", 1, &calls)).
				Plans(planTo("second", "fixture", provider("maker", "cap"))).
				Build()
			assert.HasError(t, err, "the composition fails")
			want := `annotator follows runs after capability "cap", which the generator maker provides`
			assert.Contains(t, err.Error(), want, "the error names the annotator and the generator")
		})

		t.Run("runs the generator half of a dual plugin after the generator of a capability that it requires",
			func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, dualOrder(t, caps("cap"), nil), []plugin.ID{makerID, duoID},
					"the requirement orders the generator half after the provider")
			})

		t.Run("runs the generator half of a dual plugin after the generator of a capability that it runs after",
			func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, dualOrder(t, nil, caps("cap")), []plugin.ID{makerID, duoID},
					"the capability orders the generator half after the provider")
			})

		t.Run("returns an error naming the plugins of a cycle through a followed capability", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(
				&declaring{name: "first", provides: caps("x"), after: caps("y")},
				&declaring{name: "second", provides: caps("y"), requires: caps("x")},
			).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "capabilities cycle among first and second", "the error names the cycle")
		})

		t.Run("numbers the turn of each claim by its schedule position", func(t *testing.T) {
			t.Parallel()

			var rank meta.Key[string]
			register := func(r *meta.Registry) error {
				if err := r.ClaimNamespace("order"); err != nil {
					return err
				}
				k, err := meta.Register[string](r, meta.KeySpec{
					Name: "order.rank", Doc: "which role stamped first",
				})
				rank = k
				return err
			}
			stampRank := func(name plugin.ID, pri int) plugin.Annotator {
				return stamperAt(name, pri, nil, nil,
					func(m *eidos.StructMatch, st *eidos.Stamper) error {
						eidos.Stamp(st, rank, string(name))
						return nil
					})
			}
			w, err := workspace.New().
				Brand(fixtureBrand).
				Keys(register).
				Annotators(stampRank("second", 2), stampRank("first", 1)).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the turn fixture composes")
			g, s := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the turn fixture runs")

			got, held := meta.Get(report.Facts, s.Identity(), rank)
			assert.True(t, held, "the fact is stamped")
			assert.Equal(t, got, "first", "the earlier turn outranks the later turn")
			turns := map[plugin.ID]int{}
			for view := range report.Facts.Claims(s.Identity(), rank.ID()) {
				turns[view.Claim.Plugin] = view.Claim.Turn
			}
			assert.Equal(t, turns, map[plugin.ID]int{"first": 2, "second": 3},
				"each claim has the schedule position of its plugin after the lowering entry of the target")
		})
	})

	t.Run("bindKeys", func(t *testing.T) {
		t.Parallel()

		t.Run("runs a gate on a named handle over the stamps of the key's registrant", func(t *testing.T) {
			t.Parallel()

			var visited []string
			w, err := valid().Annotators(
				flagger(caps(flagsCap)),
				auditing(eidos.HasKey(meta.Named[bool](flagKeyName)), caps(flagsCap), nil, &visited),
			).Build()
			assert.NoError(t, err, "the composition builds")
			g, _ := alpha(t)
			_, err = w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the composition runs")
			assert.Equal(t, visited, []string{"Alpha"}, "the gate admits the struct that the registrant flagged")
		})

		absent := []struct {
			name string
			gate eidos.Pred
			want []string
		}{
			{
				name: "does not select a subject through a gate on a namespace that nothing claims after a capability " +
					"without a provider",
				gate: eidos.HasKey(meta.Named[bool](absentKeyName)),
			},
			{
				name: "selects every subject through a negated gate on a namespace that nothing claims after a capability " +
					"without a provider",
				gate: eidos.Not(eidos.HasKey(meta.Named[bool](absentKeyName))),
				want: []string{"Alpha"},
			},
		}
		for _, tt := range absent {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var visited []string
				w, err := valid().Annotators(auditing(tt.gate, nil, caps(absentCap), &visited)).Build()
				assert.NoError(t, err, "the composition builds without the registrant of the key")
				g, _ := alpha(t)
				_, err = w.Run(t.Context(), workspace.Input{Graph: g})
				assert.NoError(t, err, "the composition runs")
				assert.Equal(t, visited, tt.want, "the gate reads the key as absent on every struct")
			})
		}

		refused := []struct {
			name    string
			flagger plugin.Annotator
			gate    eidos.Pred
			after   []plugin.Capability
			markers []string
		}{
			{
				name:    "returns an error naming the plugin and the key of a name that nothing registers",
				flagger: flagger(caps(flagsCap)),
				gate:    eidos.HasKey(meta.Named[bool](ghostFlagName)),
				markers: []string{string(auditID), string(ghostFlagName), "nothing registers"},
			},
			{
				name:    "returns an error naming the plugin and the key of a name with another value type",
				flagger: flagger(caps(flagsCap)),
				gate:    eidos.KeyEquals(meta.Named[string](flagKeyName), "yes"),
				markers: []string{string(auditID), string(flagKeyName), "another value type than string"},
			},
			{
				name:    "returns an error naming the capability of a registrant that runs after the gate",
				flagger: flagger(caps(flagsCap)),
				gate:    eidos.HasKey(meta.Named[bool](flagKeyName)),
				markers: []string{string(auditID), string(flagKeyName), string(flaggerID), `require "flags"`},
			},
			{
				name:    "returns an error stating that a registrant after the gate provides no capability",
				flagger: flagger(nil),
				gate:    eidos.HasKey(meta.Named[bool](flagKeyName)),
				markers: []string{string(auditID), "flagger provides no capability to require"},
			},
			{
				name:    "returns an error naming the plugin and the key of a namespace that nothing claims",
				flagger: flagger(caps(flagsCap)),
				gate:    eidos.HasKey(meta.Named[bool](absentKeyName)),
				markers: []string{string(auditID), string(absentKeyName), "nothing claims the namespace"},
			},
			{
				name:    "returns an error for a namespace that nothing claims where each capability of After has a provider",
				flagger: flagger(caps(flagsCap)),
				gate:    eidos.HasKey(meta.Named[bool](absentKeyName)),
				after:   caps(flagsCap),
				markers: []string{string(auditID), string(absentKeyName), "nothing claims the namespace"},
			},
			{
				name: "returns an error naming the key of a name that nothing registers after a capability " +
					"without a provider",
				flagger: flagger(caps(flagsCap)),
				gate:    eidos.HasKey(meta.Named[bool](ghostFlagName)),
				after:   caps(absentCap),
				markers: []string{string(auditID), string(ghostFlagName), "nothing registers"},
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var visited []string
				_, err := valid().Annotators(tt.flagger, auditing(tt.gate, nil, tt.after, &visited)).Build()
				assert.HasError(t, err, "the composition fails")
				for _, marker := range tt.markers {
					assert.Contains(t, err.Error(), marker, "the error names the fault")
				}
			})
		}

		t.Run("returns one error for a key that two rules of an annotator gate on", func(t *testing.T) {
			t.Parallel()

			twice := eidos.NewPlugin(auditID).
				Handle(eidos.Where(eidos.HasKey(meta.Named[bool](flagKeyName)),
					eidos.OnStruct(quiet),
					eidos.OnInterface(func(*eidos.InterfaceMatch, *eidos.Stamper) error { return nil }),
				)).
				Annotator()
			_, err := valid().Annotators(flagger(caps(flagsCap)), twice).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Equal(t, strings.Count(err.Error(), string(flagKeyName)), 1, "the gate on the key is refused once")
		})

		t.Run("returns the error of an unregistered name without the error of an unclaimed namespace",
			func(t *testing.T) {
				t.Parallel()

				mixed := eidos.NewPlugin(auditID).
					After(absentCap).
					Handle(
						eidos.Where(eidos.HasKey(meta.Named[bool](absentKeyName)), eidos.OnStruct(quiet)),
						eidos.Where(eidos.HasKey(meta.Named[bool](ghostFlagName)), eidos.OnStruct(quiet)),
					).
					Annotator()
				_, err := valid().Annotators(flagger(caps(flagsCap)), mixed).Build()
				assert.HasError(t, err, "the composition fails")
				expect.That(t, err.Error()).
					Contains(string(ghostFlagName), "the error names the key that nothing registers").
					NotContains(string(absentKeyName), "the error leaves out the key of the namespace that nothing claims")
			})

		t.Run("returns an error that wraps ErrUnclaimedNamespace for a plugin without capabilities",
			func(t *testing.T) {
				t.Parallel()

				_, err := valid().Annotators(unclaiming{}).Build()
				assert.ErrorIs(t, err, plugin.ErrUnclaimedNamespace, "the composition fails with the error of the bind")
			})

		t.Run("returns an error naming a plugin whose BindKeys returns a plugin of another type", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(rebinding{}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "rebinding returns a plugin of another type or another name from BindKeys",
				"the error names the plugin")
		})
	})

	t.Run("compilePlans", func(t *testing.T) {
		t.Parallel()

		t.Run("runs a plan's generators in turn order", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			placed := func(name plugin.ID, pri int) plugin.Generator {
				return generatorAt(name, pri, func(*eidos.StructMatch, *eidos.Emitter) error {
					calls = append(calls, name)
					return nil
				})
			}
			w, err := workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{placed("late", 9), placed("early", 3)},
					Backend:    fakeBackend{name: "printer", target: "fixture"},
				}).
				Build()
			assert.NoError(t, err, "the plan fixture composes")
			g, _ := alpha(t)
			_, err = w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the plan fixture runs")
			assert.Equal(t, calls, []plugin.ID{"early", "late"}, "the list order does not decide")
		})

		t.Run("returns an error naming a generator whose trees serve other targets alone", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Plans(planTo("second", "fixture", styled(eidos.NewPlugin("styled").
					For("other", eidos.Templates(slotted()))))).
				Build()
			assert.HasError(t, err, "the plan's target has no tree to resolve references in")
			assert.Contains(t, err.Error(), "styled", "the error names the generator")
			assert.Contains(t, err.Error(), `"other"`, "the error names the declared target")
		})

		t.Run("returns no error for a generator whose plugin-level tree serves every target", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Plans(planTo("second", "fixture", styled(eidos.NewPlugin("styled").
					Templates(slotted()).
					For("other", eidos.Templates(slotted()))))).
				Build()
			assert.NoError(t, err, "the plugin-level tree serves the plan's target")
		})

		t.Run("returns no error for an extensible plan without generators", func(t *testing.T) {
			t.Parallel()

			empty := planTo("second", "fixture")
			empty.Extensible = true
			w, err := valid().Plans(empty).Build()
			assert.NoError(t, err, "an extensible plan admits the config's generators and has none of its own")
			plans := w.Describe().Plans
			assert.Length(t, plans, 2, "the composition has both plans")
			assert.Empty(t, plans[1].Generate, "the extensible plan schedules no generator")
		})

		refused := []struct {
			name string
			deps []string
			want string
		}{
			{
				name: "returns an error naming a dependency the composition does not declare",
				deps: []string{"absent"},
				want: `plan "bindings" depends on "absent", which the composition does not declare`,
			},
			{
				name: "returns an error naming a plan that depends on itself",
				deps: []string{"bindings"},
				want: `plan "bindings" depends on itself`,
			},
			{
				name: "returns an error naming a dependency listed twice",
				deps: []string{"plan", "plan"},
				want: `plan "bindings" lists "plan" twice in its dependencies`,
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				bindings := planTo("bindings", "fixture", mirror("bindings-mirror"))
				bindings.DependsOn = tt.deps
				_, err := valid().Plans(bindings).Build()
				assert.HasError(t, err, "the dependency is refused")
				assert.Contains(t, err.Error(), tt.want, "the error names the plan and the dependency")
			})
		}

		t.Run("returns no error for a dependency on a declared plan", func(t *testing.T) {
			t.Parallel()

			bindings := planTo("bindings", "fixture", mirror("bindings-mirror"))
			bindings.DependsOn = []string{"plan"}
			_, err := valid().Plans(bindings).Build()
			assert.NoError(t, err, "the dependency is on the valid composition's plan")
		})
	})

	t.Run("orderPlans", func(t *testing.T) {
		t.Parallel()

		dependingOn := func(name string, deps ...string) workspace.Plan {
			p := planTo(name, "fixture", mirror(plugin.ID(name+"-mirror")))
			p.DependsOn = deps
			return p
		}

		t.Run("returns an error naming the two plans of a cycle", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(dependingOn("server", "bindings"), dependingOn("bindings", "server")).Build()
			assert.HasError(t, err, "the cycle is refused")
			assert.Contains(t, err.Error(), `plans "bindings" and "server" depend on each other in a cycle`,
				"the error names both plans")
		})

		t.Run("returns an error naming every plan of a longer cycle", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(dependingOn("a", "c"), dependingOn("b", "a"), dependingOn("c", "b")).Build()
			assert.HasError(t, err, "the cycle is refused")
			assert.Contains(t, err.Error(), `plans "a" and "b" and "c" depend on each other in a cycle`,
				"the error names every plan of the cycle")
		})

		t.Run("returns one error for each of two cycles", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(dependingOn("a", "b"), dependingOn("b", "a"),
				dependingOn("c", "d"), dependingOn("d", "c")).Build()
			assert.HasError(t, err, "both cycles are refused")
			assert.Contains(t, err.Error(), `plans "a" and "b" depend`, "the first cycle")
			assert.Contains(t, err.Error(), `plans "c" and "d" depend`, "the second cycle")
		})

		t.Run("names no plan that depends on a cycle and is not in it", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(dependingOn("a", "b"), dependingOn("b", "a"), dependingOn("tail", "a")).Build()
			assert.HasError(t, err, "the cycle is refused")
			assert.NotContains(t, err.Error(), `"tail"`, "the error names the cycle's plans alone")
		})
	})

	t.Run("compileChecks", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a nil check", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Checks(nil, &recordingCheck{name: "stubbed"}).Build()
			assert.HasError(t, err, "a nil check is refused")
			assert.Contains(t, err.Error(), "check 1 of 2 is nil", "the error counts the checks")
		})

		t.Run("returns an error naming a check that reads an undeclared plan", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Checks(&recordingCheck{name: "stubbed", reads: []string{"absent"}}).Build()
			assert.HasError(t, err, "the read is refused")
			assert.Contains(t, err.Error(), `check stubbed reads plan "absent", which the composition does not declare`,
				"the error names the check and the plan")
		})

		t.Run("returns an error naming a check that reads one plan twice", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Checks(&recordingCheck{name: "stubbed", reads: []string{"plan", "plan"}}).Build()
			assert.HasError(t, err, "the second read is refused")
			assert.Contains(t, err.Error(), `check stubbed reads plan "plan" twice`,
				"the error names the check and the plan")
		})

		t.Run("returns an error naming a check whose name another plugin has", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Checks(&recordingCheck{name: "mirror"}).Build()
			assert.HasError(t, err, "the name is taken")
			assert.Contains(t, err.Error(), `two plugins return the name "mirror"`, "the error names the plugin")
		})
	})
}

// runOrdering composes the annotators and runs them over the fixture of
// one struct, so the handler of each role runs once, in schedule order.
func runOrdering(t *testing.T, anns ...plugin.Annotator) {
	t.Helper()

	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(anns...).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirror("mirror"))).
		Build()
	assert.NoError(t, err, "the ordering fixture composes")
	g, _ := alpha(t)
	_, err = w.Run(t.Context(), workspace.Input{Graph: g})
	assert.NoError(t, err, "the ordering fixture runs")
}

// dualOrder composes the generator makerID, which provides the capability
// cap, with the plugin duoID, which requires the capabilities requires and
// runs after the capabilities after. The annotator half of duoID stamps
// every struct, and each generator records its runs. dualOrder runs the
// composition over the fixture of one struct and returns the generators in
// the order of their runs.
func dualOrder(t *testing.T, requires, after []plugin.Capability) []plugin.ID {
	t.Helper()

	var (
		calls []plugin.ID
		seen  meta.Key[bool]
	)
	maker := eidos.NewPlugin(makerID).
		Provides("cap").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "made"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
			calls = append(calls, makerID)
			return nil
		})).Generator()
	duo := eidos.NewPlugin(duoID).
		Requires(requires...).
		After(after...).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace(duoKeyName.Namespace()); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{Name: duoKeyName, Doc: "a struct that duo saw"})
			seen = k
			return err
		}).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "duo"}).
		Handle(
			eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
				eidos.Stamp(st, seen, true)
				return nil
			}),
			eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
				calls = append(calls, duoID)
				return nil
			}),
		).Dual()
	w, err := valid().Plans(planTo("second", "fixture", maker, duo)).Build()
	assert.NoError(t, err, "the dual fixture composes")
	g, _ := alpha(t)
	_, err = w.Run(t.Context(), workspace.Input{Graph: g})
	assert.NoError(t, err, "the dual fixture runs")
	return calls
}

// capable returns a hand-rolled annotator with the capability lists
// provides and requires.
func capable(name plugin.ID, provides, requires []plugin.Capability) plugin.Annotator {
	return &declaring{name: name, provides: provides, requires: requires}
}

// following returns an annotator at the priority pri that runs after the
// capability cap and records its runs into calls.
func following(name plugin.ID, pri int, calls *[]plugin.ID) plugin.Annotator {
	return eidos.NewPlugin(name).
		Priority(plugin.RoleAnnotator, pri).
		After("cap").
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Stamper) error {
			*calls = append(*calls, name)
			return nil
		})).
		Annotator()
}

// flagger returns an annotator that provides the capabilities provides,
// registers the flag key and stamps it on every struct.
func flagger(provides []plugin.Capability) plugin.Annotator {
	var flag meta.Key[bool]
	return eidos.NewPlugin(flaggerID).
		Provides(provides...).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace(flagKeyName.Namespace()); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{Name: flagKeyName, Doc: "a struct that the flagger flagged"})
			flag = k
			return err
		}).
		Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
			eidos.Stamp(st, flag, true)
			return nil
		})).
		Annotator()
}

// auditing returns an annotator that requires the capabilities requires,
// runs after the capabilities after, and appends the name of each struct
// that gate admits to visited.
func auditing(gate eidos.Pred, requires, after []plugin.Capability, visited *[]string) plugin.Annotator {
	return eidos.NewPlugin(auditID).
		Requires(requires...).
		After(after...).
		Handle(eidos.Where(gate, eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Stamper) error {
			*visited = append(*visited, m.Struct.Name)
			return nil
		}))).
		Annotator()
}

// keyed returns an annotator whose key provider runs register.
func keyed(name plugin.ID, register func(*meta.Registry) error) plugin.Annotator {
	return eidos.NewPlugin(name).
		Keys(register).
		Handle(eidos.OnStruct(quiet)).Annotator()
}

// keyedKeys claims the namespace of [keyedLang] and registers
// [keyedKey], as a frontend registers its language's keys.
func keyedKeys(r *meta.Registry) error {
	if err := r.ClaimNamespace(string(keyedLang)); err != nil {
		return err
	}
	_, err := meta.Register[bool](r, meta.KeySpec{Name: keyedKey, Doc: "marks a flagged declaration"})
	return err
}

// schemad returns a generator that declares s, so the registration step
// meets the schema of a plugin.
func schemad(name plugin.ID, s directive.Schema) plugin.Generator {
	return eidos.NewPlugin(name).
		Output(plugin.Output{Per: plugin.PerSource, Word: "gen"}).
		Handle(eidos.Directive(s, eidos.OnEmit(symbol.KindStruct,
			func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }))).
		Generator()
}

// slotted returns a tree of one template that places the slots, the
// smallest tree that the template rules admit.
func slotted() fstest.MapFS {
	return fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("{{slots}}")}}
}

// styled returns the generator that the declaration b builds, with one
// family and a rule that emits nothing.
func styled(b *eidos.Builder) plugin.Generator {
	return b.
		Output(plugin.Output{Per: plugin.PerSource, Word: "gen"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
		Generator()
}

// sectioned returns a composition with one config section for the
// options struct of one plugin.
func sectioned(name string, cfg any, section map[string]any) *workspace.Builder {
	return valid().
		Plans(planTo("second", "fixture", tuned(plugin.ID(name), cfg))).
		Config(workspace.Config{Options: map[string]map[string]any{name: section}})
}
