// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The validation fixture: a subject, a metadata registry with one
// key and one group, and a schema exercising every param shape.
var validationSubject = symbol.Identity{
	Lang: "golang", Package: "svc/store", Name: "Store", Kind: symbol.KindStruct,
}

// keyed returns a metadata registry holding shape.role in the
// group shape.writer, for drop resolution.
func keyed(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace("shape", "eidos-plugin-shape"), "the namespace claims")
	_, err := meta.Register[string](r, meta.KeySpec{
		Name: "shape.role", Group: "shape.writer", Doc: "the classified role",
	})
	assert.NoError(tb, err, "the fixture key registers")
	return r
}

// fullSchema exercises positionals, every scalar type, a reference
// list, roles and repeatability.
func fullSchema() directive.Schema {
	return directive.Schema{
		Plugin: "indexer",
		Name:   "index",
		Positional: []directive.ParamSpec{
			{Key: "kind", Type: directive.TypeString, Required: true, Doc: "the index kind"},
		},
		Params: []directive.ParamSpec{
			{
				Key: "fields", Type: directive.TypeList, ListOf: directive.TypeReference,
				Resolution: directive.ResolveValueField, Doc: "the indexed members",
			},
			{Key: "depth", Type: directive.TypeInt, Doc: "the tree depth"},
			{Key: "unique", Type: directive.TypeBool, Doc: "one row per key"},
			{
				Key: "shard", Type: directive.TypeString, Roles: []string{"server"},
				Doc: "the server-side shard key",
			},
		},
		Roles:      []string{"client", "server"},
		Repeatable: true,
		Doc:        "declares an index over members",
	}
}

// parse returns the parsed payload, failing the test on a payload
// the grammar refuses.
func parse(tb assert.TB, payload string, line int) directive.Raw {
	tb.Helper()

	raw, err := directive.Parse(payload)
	assert.NoError(tb, err, "the fixture payload parses")
	raw.Pos = position.Pos{File: "svc/store.go", Line: line, Col: 1}
	return raw
}

// validate runs Validate over payloads with the full fixture,
// returning the typed instances and the sink.
func validate(tb assert.TB, payloads ...string) ([]directive.Directive, *diag.Sink) {
	tb.Helper()

	r := sealed(tb, fullSchema(), wellFormed("stubgen", "index"))
	sink := diag.NewSink()
	raws := make([]directive.Raw, 0, len(payloads))
	for i, payload := range payloads {
		raws = append(raws, parse(tb, payload, i+1))
	}
	return directive.Validate(validationSubject, raws, r, keyed(tb), sink), sink
}

// failures returns the codes the sink holds, in report order.
func failures(sink *diag.Sink) []diag.Code {
	var out []diag.Code
	for d := range sink.All() {
		out = append(out, d.Code)
	}
	return out
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("types a full instance", func(t *testing.T) {
		t.Parallel()

		got, sink := validate(t,
			`indexer:index btree fields=[a, b] depth=3 unique=true role=server shard=id out=x.go`)
		assert.False(t, sink.Failed(), "a well-formed instance passes")
		assert.Length(t, got, 1, "and types")

		d := got[0]
		assert.Equal(t, d.Name, directive.Name("indexer:index"),
			"the name is the schema's canonical spelling")
		assert.Equal(t, d.Args, []directive.Value{
			{Kind: directive.TypeString, Str: "btree"},
		}, "positionals type per the declared order")
		fields, held := d.Param("fields")
		assert.True(t, held, "a declared list param returns")
		assert.Equal(t, fields, directive.Value{
			Kind: directive.TypeList,
			List: []directive.Value{
				{Kind: directive.TypeReference, Ref: "a"},
				{Kind: directive.TypeReference, Ref: "b"},
			},
		}, "with each element typed as the declared reference")
		depth, _ := d.Param("depth")
		assert.Equal(t, depth, directive.Value{Kind: directive.TypeInt, Int: 3}, "ints parse")
		unique, _ := d.Param("unique")
		assert.Equal(t, unique, directive.Value{Kind: directive.TypeBool, Bool: true}, "bools parse")
		assert.Equal(t, d.Role, "server", "the role is validated and carried")
		shard, _ := d.Param("shard")
		assert.Equal(t, shard.Str, "id", "a role-scoped param admits under its role")
		out, held := d.Param(directive.ReservedOut)
		assert.True(t, held, "a reserved key arrives in the params")
		assert.Equal(t, out, directive.Value{Kind: directive.TypeString, Str: "x.go"},
			"typed as a string")
		assert.Equal(t, d.Instance, 0, "the first instance is numbered zero")
	})

	t.Run("numbers repeatable instances in position order", func(t *testing.T) {
		t.Parallel()

		got, sink := validate(t,
			"indexer:index hash",
			"indexer:index btree")
		assert.False(t, sink.Failed(), "two instances of a repeatable schema pass")
		assert.Length(t, got, 2, "both type")
		assert.Equal(t, got[0].Instance, 0, "the first by position is zero")
		assert.Equal(t, got[0].Args[0].Str, "hash", "and is the earlier line")
		assert.Equal(t, got[1].Instance, 1, "the second is one")
	})

	t.Run("refuses what the checks refuse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			payloads []string
			want     diag.Code
			naming   string
		}{
			{
				name:     "an unclaimed name",
				payloads: []string{"nonexistent"},
				want:     directive.UnclaimedName,
				naming:   "nonexistent",
			},
			{
				name:     "an ambiguous bare name naming its candidates",
				payloads: []string{"index btree"},
				want:     directive.AmbiguousName,
				naming:   "indexer:index",
			},
			{
				name:     "an unknown key",
				payloads: []string{"indexer:index btree nonkey=v"},
				want:     directive.UnknownKey,
				naming:   "nonkey",
			},
			{
				name:     "a key written twice",
				payloads: []string{"indexer:index btree depth=1 depth=2"},
				want:     directive.DuplicateKey,
				naming:   "depth",
			},
			{
				name:     "a scalar where the schema says list",
				payloads: []string{"indexer:index btree fields=a"},
				want:     directive.TypeMismatch,
				naming:   "fields",
			},
			{
				name:     "a nested list",
				payloads: []string{"indexer:index btree fields=[[a]]"},
				want:     directive.TypeMismatch,
				naming:   "fields",
			},
			{
				name:     "an int outside its spelling",
				payloads: []string{"indexer:index btree depth=deep"},
				want:     directive.BadSpelling,
				naming:   "depth",
			},
			{
				name:     "a bool outside its spelling",
				payloads: []string{"indexer:index btree unique=yes"},
				want:     directive.BadSpelling,
				naming:   "unique",
			},
			{
				name:     "an argument past the declared positionals",
				payloads: []string{"indexer:index btree extra"},
				want:     directive.ExtraPositional,
				naming:   "extra",
			},
			{
				name:     "an omitted required param",
				payloads: []string{"indexer:index depth=1"},
				want:     directive.MissingParam,
				naming:   "kind",
			},
			{
				name:     "an unknown role naming the declared set",
				payloads: []string{"indexer:index btree role=admin"},
				want:     directive.UnknownRole,
				naming:   "server",
			},
			{
				name:     "a role-scoped param outside its role",
				payloads: []string{"indexer:index btree role=client shard=id"},
				want:     directive.UnknownKey,
				naming:   "shard",
			},
			{
				name:     "a role-scoped param on a bare instance",
				payloads: []string{"indexer:index btree shard=id"},
				want:     directive.UnknownKey,
				naming:   "shard",
			},
			{
				name:     "a false boolean spelling still types",
				payloads: []string{"indexer:index btree unique=false extra"},
				want:     directive.ExtraPositional,
				naming:   "extra",
			},
			{
				name:     "a role on a schema declaring none",
				payloads: []string{"stubgen:index role=client"},
				want:     directive.UnknownKey,
				naming:   "role",
			},
			{
				name:     "a role written as a list",
				payloads: []string{"indexer:index btree role=[client]"},
				want:     directive.TypeMismatch,
				naming:   "role",
			},
			{
				name:     "a list where the schema says scalar",
				payloads: []string{"indexer:index btree depth=[1]"},
				want:     directive.TypeMismatch,
				naming:   "depth",
			},
			{
				name:     "a metadata reference no key or group returns",
				payloads: []string{"meta drop=shape.nonexistent"},
				want:     directive.UnknownMetadataKey,
				naming:   "shape.role",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, sink := validate(t, tt.payloads...)
				assert.Empty(t, got, "a failing instance is not returned")
				assert.True(t, sink.Failed(), "and the run fails")
				assert.True(t, slices.Contains(failures(sink), tt.want),
					"under the check's own code")
				found := false
				for d := range sink.All() {
					found = found || strings.Contains(d.Msg, tt.naming)
				}
				assert.True(t, found, "naming what the author needs to fix")
			})
		}
	})

	t.Run("role requirements", func(t *testing.T) {
		t.Parallel()

		t.Run("a schema demanding a role refuses a bare instance", func(t *testing.T) {
			t.Parallel()

			strict := fullSchema()
			strict.Plugin = "strictgen"
			strict.RolesRequired = true
			r := sealed(t, strict)
			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "strictgen:index btree", 1)}, r, keyed(t), sink)

			assert.Empty(t, got, "the bare instance is refused")
			assert.True(t, slices.Contains(failures(sink), directive.MissingRole),
				"under the omitted-role code")
		})

		t.Run("a required param scoped to a role binds under that role alone", func(t *testing.T) {
			t.Parallel()

			scoped := directive.Schema{
				Plugin: "scopegen", Name: "expose", Doc: "exposes a half",
				Roles: []string{"client", "server"},
				Params: []directive.ParamSpec{
					{
						Key: "shard", Type: directive.TypeString, Required: true,
						Roles: []string{"server"}, Doc: "the server shard",
					},
				},
			}
			r := sealed(t, scoped)

			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "scopegen:expose role=client", 1)}, r, keyed(t), sink)
			assert.Length(t, got, 1, "the client instance passes without the server param")
			assert.False(t, sink.Failed(), "nothing required it there")

			sink = diag.NewSink()
			got = directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "scopegen:expose role=server", 1)}, r, keyed(t), sink)
			assert.Empty(t, got, "the server instance without it is refused")
			assert.True(t, slices.Contains(failures(sink), directive.MissingParam),
				"under the omitted-param code")
		})
	})

	t.Run("repeatability", func(t *testing.T) {
		t.Parallel()

		t.Run("a second instance of a single-instance schema names both positions", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t,
				"stubgen:index mode=a",
				"stubgen:index mode=b")
			assert.Empty(t, got, "the contradiction is refused whole")
			assert.True(t, slices.Contains(failures(sink), directive.DuplicateInstance),
				"under the duplicate-instance code")
			var related int
			for d := range sink.All() {
				related += len(d.Related)
			}
			assert.True(t, related > 0, "the report carries the other position")
		})
	})

	t.Run("cross-directive constraints", func(t *testing.T) {
		t.Parallel()

		t.Run("a requirement is met by canonical schema whatever the spelling", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("weaver", "weave")
			needs.Requires = []directive.Name{"indexer:index"}
			r := sealed(t, fullSchema(), needs)

			sink := diag.NewSink()
			got := directive.Validate(validationSubject, []directive.Raw{
				parse(t, "weaver:weave", 1),
				parse(t, "indexer:index btree", 2),
			}, r, keyed(t), sink)
			assert.Length(t, got, 2, "the requirement is met")
			assert.False(t, sink.Failed(), "and nothing reports")
		})

		t.Run("an unmet requirement reports the requiring position", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("weaver", "weave")
			needs.Requires = []directive.Name{"indexer:index"}
			r := sealed(t, fullSchema(), needs)

			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "weaver:weave", 1)}, r, keyed(t), sink)
			assert.Empty(t, got, "the unmet requirement refuses the instance")
			assert.True(t, slices.Contains(failures(sink), directive.RequirementUnmet),
				"under the requirement code")
		})

		t.Run("a conflict is symmetric and names both positions", func(t *testing.T) {
			t.Parallel()

			hates := wellFormed("weaver", "weave")
			hates.ConflictsWith = []directive.Name{"indexer:index"}
			r := sealed(t, fullSchema(), hates)

			sink := diag.NewSink()
			got := directive.Validate(validationSubject, []directive.Raw{
				parse(t, "indexer:index btree", 1),
				parse(t, "weaver:weave", 2),
			}, r, keyed(t), sink)
			assert.Empty(t, got, "the pair is refused whole")
			assert.True(t, slices.Contains(failures(sink), directive.Conflict),
				"under the conflict code")
			var related int
			for d := range sink.All() {
				related += len(d.Related)
			}
			assert.True(t, related > 0, "the report carries the other position")
		})
	})

	t.Run("meta drop resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("a key and a group both resolve", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t, "meta drop=shape.role")
			assert.Length(t, got, 1, "a registered key resolves")
			assert.False(t, sink.Failed(), "without a report")

			got, sink = validate(t, "meta drop=shape.writer")
			assert.Length(t, got, 1, "and so does a registered group")
			assert.False(t, sink.Failed(), "without a report")
		})
	})

	t.Run("validating nothing returns nothing", func(t *testing.T) {
		t.Parallel()

		r := sealed(t)
		sink := diag.NewSink()
		got := directive.Validate(validationSubject, nil, r, keyed(t), sink)
		assert.Nil(t, got, "no instances, no answer")
		assert.False(t, sink.Failed(), "and no report")
	})

	t.Run("a conflict declared against an absent schema stays quiet", func(t *testing.T) {
		t.Parallel()

		hates := wellFormed("weaver", "weave")
		hates.ConflictsWith = []directive.Name{"indexer:index"}
		r := sealed(t, fullSchema(), hates)

		sink := diag.NewSink()
		got := directive.Validate(validationSubject,
			[]directive.Raw{parse(t, "weaver:weave", 1)}, r, keyed(t), sink)
		assert.Length(t, got, 1, "a conflict needs both sides present")
		assert.False(t, sink.Failed(), "so nothing reports")
	})

	t.Run("an unsealed registry refuses validation", func(t *testing.T) {
		t.Parallel()

		r := directive.NewRegistry()
		assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the schema registers")
		sink := diag.NewSink()
		got := directive.Validate(validationSubject,
			[]directive.Raw{parse(t, "mockgen:stub", 1)}, r, keyed(t), sink)

		assert.Empty(t, got, "nothing validates against a moving registry")
		assert.True(t, sink.Failed(), "and the defect reports rather than passing silently")
	})

	t.Run("every failure is a positioned error from the freeze phase", func(t *testing.T) {
		t.Parallel()

		_, sink := validate(t, "nonexistent")
		for d := range sink.All() {
			assert.Equal(t, d.Severity, diag.SeverityError, "validation reports Errors")
			assert.False(t, d.Pos.IsZero(), "every report is positioned")
			assert.Equal(t, d.Origin, diag.PhaseFreeze, "from the freeze phase")
		}
	})
}

// Validation runs once per subject at the seal, so its cost per
// subject bounds the freeze-adjacent pass over a large workspace.
func BenchmarkValidate(b *testing.B) {
	b.ReportAllocs()

	r := sealed(b, fullSchema(), wellFormed("stubgen", "index"))
	keys := keyed(b)
	raws := []directive.Raw{
		parse(b, `indexer:index btree fields=[a, b] depth=3 unique=true role=server shard=id`, 1),
		parse(b, "stubgen:index mode=fast", 2),
		parse(b, "meta drop=shape.role", 3),
	}

	for b.Loop() {
		sink := diag.NewSink()
		if got := directive.Validate(validationSubject, raws, r, keys, sink); len(got) != 3 {
			b.Fatalf("Validate returned %d instances, want 3", len(got))
		}
	}
}
