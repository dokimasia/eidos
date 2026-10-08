// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The validation fixture: a subject, a metadata registry with one
// key and one group, and a schema exercising every param shape.
var validationSubject = symbol.Identity{
	Lang: "golang", Package: "svc/store", Name: "Store", Kind: symbol.KindStruct,
}

// The validation benchmark's instances: an index with a positional, a
// list and three params under a role, a second plugin's index, and a
// drop of a metadata key.
const (
	fullIndexPayload  = `indexer:index btree fields=[a, b] depth=3 unique=true role=server shard=id`
	otherIndexPayload = "stubgen:index mode=fast"
	dropPayload       = "meta drop=shape.role"
)

// unregisteredCode is a code in the kernel's prefix that no package
// registers.
const unregisteredCode = "EID-9999"

// validateAllocs is what validating the benchmark's three instances
// allocates: the returned slice, and per instance the params map, its
// one group and each param's value, which the map keeps apart because a
// Value is larger than 128 bytes. The index also allocates its
// positional and its list.
const validateAllocs = 1 + (2 + 4 + 2) + (2 + 1) + (2 + 1)

// Validate is the one gate between a carrier and a handler, so every
// check it runs and every finding it reports is pinned.
func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("Validate", func(t *testing.T) {
		t.Parallel()

		t.Run("reports MixedCarriers for a repeatable directive in two carrier shapes", func(t *testing.T) {
			t.Parallel()

			_, sink := mixedIndexes(t)
			coretest.AssertCodes(t, sink, directive.MixedCarriers)
		})

		t.Run("reports MixedCarriers once for a repeatable directive in two carrier shapes", func(t *testing.T) {
			t.Parallel()

			_, sink := validateRaws(t, []directive.Raw{
				parse(t, "indexer:index hash", 1),
				shaped(t, "indexer:index btree", 2),
				shaped(t, "indexer:index gin", 3),
			}, fullSchema())
			coretest.AssertCodes(t, sink, directive.MixedCarriers)
		})

		t.Run("reports MixedCarriers as a Warning", func(t *testing.T) {
			t.Parallel()

			_, sink := mixedIndexes(t)
			assert.Equal(t, onlyDiag(t, sink).Severity, diag.SeverityWarning, "a formatter's reordering fails nothing")
		})

		t.Run("relates MixedCarriers to the first instance", func(t *testing.T) {
			t.Parallel()

			_, sink := mixedIndexes(t)
			assert.Equal(t, onlyDiag(t, sink).Related, []position.Pos{{File: "svc/store.go", Line: 1, Col: 1}},
				"the finding names the instance of the other shape")
		})

		t.Run("keeps every instance of a repeatable directive in two carrier shapes", func(t *testing.T) {
			t.Parallel()

			got, _ := mixedIndexes(t)
			assert.Length(t, got, 2, "the warning drops nothing")
		})

		t.Run("reports nothing for a repeatable directive in the tool-directive shape alone", func(t *testing.T) {
			t.Parallel()

			_, sink := validateRaws(t, []directive.Raw{
				shaped(t, "indexer:index hash", 1),
				shaped(t, "indexer:index btree", 2),
			}, fullSchema())
			coretest.AssertCodes(t, sink)
		})

		t.Run("returns a full instance typed per its schema", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t,
				`indexer:index btree fields=[a, b] depth=3 unique=true role=server shard=id out=x.go`)
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "one instance is returned")

			d := got[0]
			assert.Equal(t, d.Name, directive.Name("indexer:index"),
				"the name is the schema's canonical spelling")
			assert.Equal(t, d.Args, []directive.Value{
				{Kind: directive.TypeString, Str: "btree"},
			}, "the positionals type in the declared order")
			fields, held := d.Param("fields")
			assert.True(t, held, "the list param is present")
			assert.Equal(t, fields, directive.Value{
				Kind: directive.TypeList,
				List: []directive.Value{
					{Kind: directive.TypeReference, Ref: "a"},
					{Kind: directive.TypeReference, Ref: "b"},
				},
			}, "each element types as the declared reference")
			depth, _ := d.Param("depth")
			assert.Equal(t, depth, directive.Value{Kind: directive.TypeInt, Int: 3}, "the int parses")
			unique, _ := d.Param("unique")
			assert.Equal(t, unique, directive.Value{Kind: directive.TypeBool, Bool: true}, "the bool parses")
			assert.Equal(t, d.Role, "server", "the role is validated")
			shard, _ := d.Param("shard")
			assert.Equal(t, shard.Str, "id", "the role-scoped param is admitted under its role")
			out, held := d.Param(directive.ReservedOut)
			assert.True(t, held, "the reserved key is in the params")
			assert.Equal(t, out, directive.Value{Kind: directive.TypeString, Str: "x.go"},
				"the reserved key types as a string")
			assert.Equal(t, d.Instance, 0, "the first instance is numbered zero")
			assert.False(t, d.Negated, "the instance is set, not negated")
		})

		t.Run("drops an ignored name without a finding", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.Total(t, r.Register, directive.Kernel(), "the kernel schemas register first")
			assert.NoError(t, r.Register(fullSchema()), "the fixture schema registers")
			assert.NoError(t, r.Ignore("k8s:"), "the workspace opts out of a foreign tool's prefix")
			assert.Empty(t, r.Seal(), "the registry seals")

			sink := diag.NewSink()
			got := directive.Validate(validationSubject, []directive.Raw{
				parse(t, "k8s:deepcopy-gen package", 1),
				parse(t, "indexer:index btree", 2),
			}, r, keyed(t), nil, sink)
			coretest.AssertCodes(t, sink)
			assert.Length(t, got, 1, "only the claimed instance is returned")
			assert.Equal(t, got[0].Name, directive.Name("indexer:index"), "the instance is the claimed one")
		})

		t.Run("numbers repeatable instances in position order", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t,
				"indexer:index hash",
				"indexer:index btree")
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 2, "both instances are returned")
			assert.Equal(t, got[0].Instance, 0, "the first by position is zero")
			assert.Equal(t, got[0].Args[0].Str, "hash", "the first by position is the earlier line")
			assert.Equal(t, got[1].Instance, 1, "the second by position is one")
		})

		tests := []struct {
			name     string
			payloads []string
			want     diag.Code
			naming   string
		}{
			{
				name:     "reports UnclaimedName naming an unclaimed name",
				payloads: []string{"nonexistent"},
				want:     directive.UnclaimedName,
				naming:   "nonexistent",
			},
			{
				name:     "reports AmbiguousName naming the candidates of an ambiguous bare name",
				payloads: []string{"index btree"},
				want:     directive.AmbiguousName,
				naming:   "indexer:index",
			},
			{
				name:     "reports UnknownKey naming an unknown key",
				payloads: []string{"indexer:index btree nonkey=v"},
				want:     directive.UnknownKey,
				naming:   "nonkey",
			},
			{
				name:     "reports DuplicateKey naming a key written twice",
				payloads: []string{"indexer:index btree depth=1 depth=2"},
				want:     directive.DuplicateKey,
				naming:   "depth",
			},
			{
				name:     "reports TypeMismatch for a scalar where the schema declares a list",
				payloads: []string{"indexer:index btree fields=a"},
				want:     directive.TypeMismatch,
				naming:   "fields",
			},
			{
				name:     "reports TypeMismatch for a nested list",
				payloads: []string{"indexer:index btree fields=[[a]]"},
				want:     directive.TypeMismatch,
				naming:   "fields",
			},
			{
				name:     "reports BadSpelling for an int outside its spelling",
				payloads: []string{"indexer:index btree depth=deep"},
				want:     directive.BadSpelling,
				naming:   "depth",
			},
			{
				name:     "reports BadSpelling for a bool outside its spelling",
				payloads: []string{"indexer:index btree unique=yes"},
				want:     directive.BadSpelling,
				naming:   "unique",
			},
			{
				name:     "reports ExtraPositional for an argument past the declared positionals",
				payloads: []string{"indexer:index btree extra"},
				want:     directive.ExtraPositional,
				naming:   "extra",
			},
			{
				name:     "reports TypeMismatch for a list where a positional takes a scalar",
				payloads: []string{"indexer:index [a, b]"},
				want:     directive.TypeMismatch,
				naming:   "kind",
			},
			{
				name:     "reports ExtraPositional for an extra positional written as a list",
				payloads: []string{"indexer:index btree [a, b]"},
				want:     directive.ExtraPositional,
				naming:   "a list",
			},
			{
				name:     "reports MissingParam for an omitted required param",
				payloads: []string{"indexer:index depth=1"},
				want:     directive.MissingParam,
				naming:   "kind",
			},
			{
				name:     "reports UnknownRole naming the declared roles",
				payloads: []string{"indexer:index btree role=admin"},
				want:     directive.UnknownRole,
				naming:   "server",
			},
			{
				name:     "reports UnknownKey for a role-scoped param outside its role",
				payloads: []string{"indexer:index btree role=client shard=id"},
				want:     directive.UnknownKey,
				naming:   "shard",
			},
			{
				name:     "reports UnknownKey for a role-scoped param on an instance without a role",
				payloads: []string{"indexer:index btree shard=id"},
				want:     directive.UnknownKey,
				naming:   "shard",
			},
			{
				name:     "reports ExtraPositional after typing a false boolean",
				payloads: []string{"indexer:index btree unique=false extra"},
				want:     directive.ExtraPositional,
				naming:   "extra",
			},
			{
				name:     "reports UnknownKey for a role on a schema that declares none",
				payloads: []string{"stubgen:index role=client"},
				want:     directive.UnknownKey,
				naming:   "role",
			},
			{
				name:     "reports TypeMismatch for a role written as a list",
				payloads: []string{"indexer:index btree role=[client]"},
				want:     directive.TypeMismatch,
				naming:   "role",
			},
			{
				name:     "reports TypeMismatch for a list where the schema declares a scalar",
				payloads: []string{"indexer:index btree depth=[1]"},
				want:     directive.TypeMismatch,
				naming:   "depth",
			},
			{
				name:     "reports UnknownMetadataKey naming the registered keys",
				payloads: []string{"meta drop=shape.nonexistent"},
				want:     directive.UnknownMetadataKey,
				naming:   "shape.role",
			},
			{
				name:     "reports UnknownCode naming a code that does not parse",
				payloads: []string{"diag off=sixty-two"},
				want:     directive.UnknownCode,
				naming:   "sixty-two",
			},
			{
				name:     "reports UnknownCode naming a code that nothing registered",
				payloads: []string{"diag off=" + unregisteredCode},
				want:     directive.UnknownCode,
				naming:   unregisteredCode,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, sink := validate(t, tt.payloads...)
				assert.Empty(t, got, "the failing instance is not returned")
				coretest.AssertReports(t, sink, tt.want)
				var msgs []string
				for d := range sink.All() {
					msgs = append(msgs, d.Msg)
				}
				assert.Contains(t, strings.Join(msgs, "\n"), tt.naming, "a finding names what the author needs to fix")
			})
		}

		t.Run("reports MissingRole for an instance without a role on a schema that demands one", func(t *testing.T) {
			t.Parallel()

			strict := fullSchema()
			strict.Plugin = "strictgen"
			strict.RolesRequired = true
			got, sink := validateRaws(t, []directive.Raw{parse(t, "strictgen:index btree", 1)}, strict)
			assert.Empty(t, got, "the instance is not returned")
			coretest.AssertReports(t, sink, directive.MissingRole)
		})

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

		t.Run("types an instance without a role-scoped required param under another role", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "scopegen:expose role=client", 1)}, scoped)
			assert.Length(t, got, 1, "the client instance is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("reports MissingParam for a role-scoped required param omitted under its role", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "scopegen:expose role=server", 1)}, scoped)
			assert.Empty(t, got, "the server instance is not returned")
			coretest.AssertReports(t, sink, directive.MissingParam)
		})

		t.Run("reports DuplicateInstance for a single-instance schema written twice", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t,
				"stubgen:index mode=a",
				"stubgen:index mode=b")
			assert.Empty(t, got, "neither instance is returned")
			coretest.AssertReports(t, sink, directive.DuplicateInstance)
			assert.NotEmpty(t, related(sink), "the finding names the other position")
		})

		t.Run("returns both instances when a requirement is met", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("weaver", "weave")
			needs.Requires = []directive.Name{"indexer:index"}
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "weaver:weave", 1),
				parse(t, "indexer:index btree", 2),
			}, fullSchema(), needs)
			assert.Length(t, got, 2, "both instances are returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("reports RequirementUnmet at the requiring position", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("weaver", "weave")
			needs.Requires = []directive.Name{"indexer:index"}
			got, sink := validateRaws(t, []directive.Raw{parse(t, "weaver:weave", 1)}, fullSchema(), needs)
			assert.Empty(t, got, "the requiring instance is not returned")
			coretest.AssertReports(t, sink, directive.RequirementUnmet)
		})

		t.Run("reports Conflict naming both positions for a conflicting pair", func(t *testing.T) {
			t.Parallel()

			hates := wellFormed("weaver", "weave")
			hates.ConflictsWith = []directive.Name{"indexer:index"}
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "indexer:index btree", 1),
				parse(t, "weaver:weave", 2),
			}, fullSchema(), hates)
			assert.Empty(t, got, "neither instance is returned")
			coretest.AssertReports(t, sink, directive.Conflict)
			assert.NotEmpty(t, related(sink), "the finding names the other position")
		})

		t.Run("reports one Conflict for a pair both schemas declare", func(t *testing.T) {
			t.Parallel()

			hates := wellFormed("weaver", "weave")
			hates.ConflictsWith = []directive.Name{"indexer:index"}
			hated := fullSchema()
			hated.ConflictsWith = []directive.Name{"weaver:weave"}
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "indexer:index btree", 1),
				parse(t, "weaver:weave", 2),
			}, hated, hates)
			assert.Empty(t, got, "neither instance is returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.Conflict}, "one Conflict is reported")
		})

		t.Run("reports nothing for a conflict with a directive the subject does not have", func(t *testing.T) {
			t.Parallel()

			hates := wellFormed("weaver", "weave")
			hates.ConflictsWith = []directive.Name{"indexer:index"}
			got, sink := validateRaws(t, []directive.Raw{parse(t, "weaver:weave", 1)}, fullSchema(), hates)
			assert.Length(t, got, 1, "the instance is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("resolves a meta drop naming a key", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t, "meta drop=shape.role")
			assert.Length(t, got, 1, "the instance is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("resolves a meta drop naming a group", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t, "meta drop=shape.writer")
			assert.Length(t, got, 1, "the instance is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("resolves a diag directive naming a registered code", func(t *testing.T) {
			t.Parallel()

			spelled := directive.UnclaimedName.String()
			got, sink := validate(t, "diag off="+spelled)
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the instance is returned")
			off, _ := got[0].Param(directive.DiagOff)
			assert.Equal(t, off, directive.Value{Kind: directive.TypeReference, Ref: spelled},
				"the code is kept as it was written")
		})

		t.Run("reports in one order whatever order the maps iterate in", func(t *testing.T) {
			t.Parallel()

			gate := directive.Schema{
				Plugin: "scoped", Name: "gate", Doc: "gates two server-side keys",
				Roles: []string{"client", "server"},
				Params: []directive.ParamSpec{
					{Key: "alpha", Type: directive.TypeString, Roles: []string{"server"}, Doc: "the first key"},
					{Key: "beta", Type: directive.TypeString, Roles: []string{"server"}, Doc: "the second key"},
				},
			}
			r := sealed(t, gate, wellFormed("weaver", "weave"), wellFormed("mockgen", "stub"))
			raws := []directive.Raw{
				parse(t, "weaver:weave", 1),
				parse(t, "mockgen:stub", 2),
				parse(t, "weaver:weave", 3),
				parse(t, "mockgen:stub", 4),
				parse(t, "scoped:gate role=client beta=b alpha=a", 5),
			}
			var first []string
			for range 20 {
				sink := diag.NewSink()
				directive.Validate(validationSubject, raws, r, keyed(t), nil, sink)
				var got []string
				for d := range sink.All() {
					got = append(got, d.Msg)
				}
				if first == nil {
					first = got
				}
				assert.Equal(t, got, first, "every validation of one subject reports in one order")
			}
			assert.Length(t, first, 4, "two duplicate pairs and two keys the role rejects are reported")
			assert.ContainsInOrder(t, strings.Join(first, "\n"),
				[]string{"alpha", "beta", "weaver:weave appears twice", "mockgen:stub appears twice"},
				"typing findings come first in key order, then the duplicates in position order")
		})

		t.Run("reports no MissingParam for a value that fails typing", func(t *testing.T) {
			t.Parallel()

			required := directive.Schema{
				Plugin: "sizer", Name: "size", Doc: "sizes a buffer",
				Positional: []directive.ParamSpec{
					{Key: "unit", Type: directive.TypeString, Required: true, Doc: "the size unit"},
				},
				Params: []directive.ParamSpec{
					{Key: "depth", Type: directive.TypeInt, Required: true, Doc: "the size"},
				},
			}
			got, sink := validateRaws(t, []directive.Raw{parse(t, "sizer:size [kb] depth=deep", 1)}, required)
			assert.Empty(t, got, "the instance is not returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.TypeMismatch, directive.BadSpelling},
				"only the two typing failures are reported")
		})

		retired := directive.Schema{
			Plugin: "sizer", Name: "limit", Doc: "bounds a buffer",
			Deprecated: "write sizer:bound in place of sizer:limit",
		}
		sizing := directive.Schema{
			Plugin: "sizer", Name: "size", Doc: "sizes a buffer",
			Positional: []directive.ParamSpec{{
				Key: "unit", Type: directive.TypeString, Doc: "the size unit",
				Deprecated: "write unit= in place of the positional unit",
			}},
			Params: []directive.ParamSpec{
				{
					Key: "limit", Type: directive.TypeInt, Doc: "the old bound",
					Deprecated: "write bound= in place of limit=",
				},
				{Key: "bound", Type: directive.TypeInt, Doc: "the bound"},
			},
		}
		deprecations := []struct {
			name    string
			payload string
			naming  string
		}{
			{
				name:    "reports DeprecatedDirective with the rewrite for a deprecated directive",
				payload: "sizer:limit",
				naming:  retired.Deprecated,
			},
			{
				name:    "reports DeprecatedDirective with the rewrite for a deprecated keyed param",
				payload: "sizer:size limit=3",
				naming:  sizing.Params[0].Deprecated,
			},
			{
				name:    "reports DeprecatedDirective with the rewrite for a deprecated positional param",
				payload: "sizer:size kb",
				naming:  sizing.Positional[0].Deprecated,
			},
		}
		for _, tt := range deprecations {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := validateRaws(t, []directive.Raw{parse(t, tt.payload, 1)}, retired, sizing)
				got := onlyDiag(t, sink)
				expect.Equal(t, got.Code, directive.DeprecatedDirective, "the finding is under its code")
				expect.That(t, got.Msg).Contains(tt.naming, "and states the rewrite")
			})
		}

		t.Run("reports DeprecatedDirective as a Warning", func(t *testing.T) {
			t.Parallel()

			_, sink := validateRaws(t, []directive.Raw{parse(t, "sizer:limit", 1)}, retired, sizing)
			assert.Equal(t, onlyDiag(t, sink).Severity, diag.SeverityWarning, "a deprecation fails nothing")
		})

		t.Run("returns the instance of a deprecated directive", func(t *testing.T) {
			t.Parallel()

			got, _ := validateRaws(t, []directive.Raw{parse(t, "sizer:limit", 1)}, retired, sizing)
			assert.Length(t, got, 1, "the deprecated instance validates as before")
		})

		t.Run("reports nothing for an instance that writes no deprecated param", func(t *testing.T) {
			t.Parallel()

			_, sink := validateRaws(t, []directive.Raw{parse(t, "sizer:size bound=3", 1)}, retired, sizing)
			coretest.AssertCodes(t, sink)
		})

		t.Run("returns nil for no instances", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, nil)
			assert.Nil(t, got, "nothing is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("reports UnsealedRegistry for an unsealed registry", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the schema registers")
			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "mockgen:stub", 1)}, r, keyed(t), nil, sink)
			assert.Empty(t, got, "nothing is returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.UnsealedRegistry},
				"UnsealedRegistry is reported")
		})

		t.Run("reports every finding as a positioned Error from the freeze phase", func(t *testing.T) {
			t.Parallel()

			_, sink := validate(t, "nonexistent")
			findings := slices.Collect(sink.All())
			assert.NotEmpty(t, findings, "the unknown directive is reported")
			for _, d := range findings {
				expect.Equal(t, d.Severity, diag.SeverityError, "the finding is an Error")
				expect.NotEqual(t, d.Pos, position.Pos{}, "the finding is positioned")
				expect.Equal(t, d.Origin, diag.PhaseFreeze, "the finding is from the freeze phase")
			}
		})

		weaveAgainstIndex := wellFormed("weaver", "weave")
		weaveAgainstIndex.ConflictsWith = []directive.Name{"indexer:index"}
		relating := []struct {
			name    string
			raws    func(tb assert.TB) []directive.Raw
			schemas []directive.Schema
			want    diag.Code
		}{
			{
				name: "reports DuplicateInstance as an Error",
				raws: func(tb assert.TB) []directive.Raw {
					return []directive.Raw{parse(tb, "mockgen:stub", 1), parse(tb, "mockgen:stub", 2)}
				},
				schemas: []directive.Schema{wellFormed("mockgen", "stub")},
				want:    directive.DuplicateInstance,
			},
			{
				name: "reports a declared Conflict as an Error",
				raws: func(tb assert.TB) []directive.Raw {
					return []directive.Raw{parse(tb, "indexer:index btree", 1), parse(tb, "weaver:weave", 2)}
				},
				schemas: []directive.Schema{fullSchema(), weaveAgainstIndex},
				want:    directive.Conflict,
			},
			{
				name: "reports the Conflict of a negated directive the subject also sets as an Error",
				raws: func(tb assert.TB) []directive.Raw {
					return []directive.Raw{parse(tb, "mockgen:stub", 1), negated(tb, "mockgen:stub", 2)}
				},
				schemas: []directive.Schema{negatable("mockgen", "stub")},
				want:    directive.Conflict,
			},
		}
		for _, tt := range relating {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := validateRaws(t, tt.raws(t), tt.schemas...)
				got := onlyDiag(t, sink)
				assert.Equal(t, got.Code, tt.want, "the finding is under its code")
				assert.Equal(t, got.Severity, diag.SeverityError, "the finding fails the run")
			})
		}

		bind := directive.Schema{
			Plugin: "witnessy", Name: "bind",
			Open:  &directive.ParamSpec{Type: directive.TypeInt, Doc: "a bound"},
			Roles: []string{"client", "server"},
			Doc:   "binds keys the instance names",
		}
		serverBind := bind
		serverBind.Open = &directive.ParamSpec{
			Type: directive.TypeInt, Roles: []string{"server"}, Doc: "a server bound",
		}

		t.Run("types undeclared keys by the open spec", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "witnessy:bind T=1 U=2 out=x", 1)}, bind)
			assert.Length(t, got, 1, "the instance is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
			v, held := got[0].Param("T")
			assert.True(t, held, "T is typed")
			expect.Equal(t, v.Kind, directive.TypeInt, "T types as the spec declares")
			expect.Equal(t, v.Int, int64(1), "T keeps its value")
			v, held = got[0].Param("U")
			assert.True(t, held, "U is typed")
			expect.Equal(t, v.Int, int64(2), "U types as the spec declares")
			v, held = got[0].Param(directive.ReservedOut)
			assert.True(t, held, "the reserved key is typed")
			expect.Equal(t, v.Kind, directive.TypeString, "the reserved key keeps its meaning")
		})

		t.Run("reports BadSpelling for an open value outside the spec's type", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "witnessy:bind T=one", 1)}, bind)
			assert.Empty(t, got, "the instance is not returned")
			coretest.AssertReports(t, sink, directive.BadSpelling)
		})

		t.Run("reports UnknownKey for an open key outside the spec's roles", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "witnessy:bind role=client T=1", 1)}, serverBind)
			assert.Empty(t, got, "the client instance is not returned")
			coretest.AssertReports(t, sink, directive.UnknownKey)
		})

		t.Run("types an open key under the spec's role", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "witnessy:bind role=server T=1", 1)}, serverBind)
			assert.Length(t, got, 1, "the server instance is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("reports UnknownKey for an undeclared key on a closed schema", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "closed:keep T=1", 1)}, wellFormed("closed", "keep"))
			assert.Empty(t, got, "the instance is not returned")
			coretest.AssertReports(t, sink, directive.UnknownKey)
		})

		field := func(name string) symbol.Identity {
			return symbol.Identity{Lang: "golang", Package: "svc/store", Name: name, Kind: symbol.KindField}
		}

		t.Run("binds a source reference through the resolver", func(t *testing.T) {
			t.Parallel()

			var askedKind directive.ResolutionKind
			var askedSubject symbol.Identity
			resolve := func(subject symbol.Identity, name string, kind directive.ResolutionKind) (symbol.Identity, error) {
				askedSubject, askedKind = subject, kind
				return field(name), nil
			}
			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "indexer:index btree fields=[id, name]", 1)},
				sealed(t, fullSchema()), keyed(t), resolve, sink)
			assert.Length(t, got, 1, "the instance is returned")
			assert.Equal(t, askedSubject, validationSubject, "the resolver is asked from the subject")
			assert.Equal(t, askedKind, directive.ResolveValueField, "the resolver is asked at the declared kind")
			fields, _ := got[0].Param("fields")
			assert.Length(t, fields.List, 2, "each element binds")
			assert.Equal(t, fields.List[0].Target, field("id"), "the target is what the resolver returned")
			assert.Equal(t, fields.List[0].Ref, "id", "the spelling is kept beside the target")
		})

		t.Run("reports UnresolvedReference with the resolver's reason", func(t *testing.T) {
			t.Parallel()

			resolve := func(_ symbol.Identity, name string, _ directive.ResolutionKind) (symbol.Identity, error) {
				return symbol.Identity{}, errors.New("no field named " + name)
			}
			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "indexer:index btree fields=[ghost]", 1)},
				sealed(t, fullSchema()), keyed(t), resolve, sink)
			assert.Empty(t, got, "the instance is not returned")
			f := onlyDiag(t, sink)
			assert.Equal(t, f.Code, directive.UnresolvedReference, "UnresolvedReference is reported")
			assert.Contains(t, f.Msg, "a field on the subject's type", "the finding names the kind")
			assert.Contains(t, f.Msg, "no field named ghost", "the finding names the resolver's reason")
		})

		t.Run("leaves a source reference unbound without a resolver", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t, "indexer:index btree fields=[id]")
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the instance is returned")
			fields, _ := got[0].Param("fields")
			assert.Length(t, fields.List, 1, "the element is typed")
			assert.Equal(t, fields.List[0].Target, symbol.Identity{}, "the target is zero")
		})

		t.Run("never asks the resolver for a metadata key", func(t *testing.T) {
			t.Parallel()

			resolve := func(symbol.Identity, string, directive.ResolutionKind) (symbol.Identity, error) {
				return symbol.Identity{}, errors.New("asked")
			}
			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "meta drop=shape.role", 1)},
				sealed(t, fullSchema()), keyed(t), resolve, sink)
			assert.Length(t, got, 1, "the metadata registry resolves the key")
			assert.False(t, sink.Failed(), "the resolver was never asked")
		})

		t.Run("returns a negated instance of a negatable schema", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{negated(t, "mockgen:stub mode=fast", 1)},
				negatable("mockgen", "stub"))
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the instance is returned")
			assert.True(t, got[0].Negated, "the instance is negated")
			mode, _ := got[0].Param("mode")
			assert.Equal(t, mode.Str, "fast", "the negated instance types its params")
		})

		t.Run("reports NegationRefused for a negated instance of a schema that is not negatable", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{negated(t, "mockgen:stub", 1)},
				wellFormed("mockgen", "stub"))
			assert.Empty(t, got, "the instance is not returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.NegationRefused},
				"NegationRefused is reported")
		})

		t.Run("reports NegationRefused for a negated kernel directive", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{negated(t, string(directive.KernelSkip), 1)})
			assert.Empty(t, got, "the instance is not returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.NegationRefused},
				"NegationRefused is reported")
		})

		t.Run("returns nothing for a negated directive the subject also sets", func(t *testing.T) {
			t.Parallel()

			got, _ := validateRaws(t, []directive.Raw{
				parse(t, "mockgen:stub", 1),
				negated(t, "mockgen:stub", 2),
			}, negatable("mockgen", "stub"))
			assert.Empty(t, got, "neither instance is returned")
		})

		t.Run("reports Conflict at the negation for a negated directive the subject also sets", func(t *testing.T) {
			t.Parallel()

			_, sink := validateRaws(t, []directive.Raw{
				parse(t, "mockgen:stub", 1),
				negated(t, "mockgen:stub", 2),
			}, negatable("mockgen", "stub"))
			f := onlyDiag(t, sink)
			assert.Equal(t, f.Code, directive.Conflict, "Conflict is reported")
			assert.Equal(t, f.Pos.Line, 2, "the finding is at the negation")
			assert.Equal(t, f.Related[0].Line, 1, "the finding names the set instance")
		})

		t.Run("reports DuplicateInstance for a single-instance schema negated twice", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{
				negated(t, "mockgen:stub", 1),
				negated(t, "mockgen:stub", 2),
			}, negatable("mockgen", "stub"))
			assert.Empty(t, got, "neither instance is returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.DuplicateInstance},
				"DuplicateInstance is reported")
		})

		t.Run("skips the requirements a negated instance's schema declares", func(t *testing.T) {
			t.Parallel()

			needs := negatable("weaver", "weave")
			needs.Requires = []directive.Name{"indexer:index"}
			got, sink := validateRaws(t, []directive.Raw{negated(t, "weaver:weave", 1)}, fullSchema(), needs)
			assert.Length(t, got, 1, "the negated instance is returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("meets no requirement with a negated instance", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("weaver", "weave")
			needs.Requires = []directive.Name{"mockgen:stub"}
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "weaver:weave", 1),
				negated(t, "mockgen:stub", 2),
			}, negatable("mockgen", "stub"), needs)
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.RequirementUnmet},
				"RequirementUnmet is reported")
			assert.Length(t, got, 1, "only the negated instance is returned")
		})

		t.Run("skips the conflicts a negated instance's schema declares", func(t *testing.T) {
			t.Parallel()

			hates := negatable("weaver", "weave")
			hates.ConflictsWith = []directive.Name{"indexer:index"}
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "indexer:index btree", 1),
				negated(t, "weaver:weave", 2),
			}, fullSchema(), hates)
			assert.Length(t, got, 2, "both instances are returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})

		t.Run("skips a conflict another schema declares against a negated instance", func(t *testing.T) {
			t.Parallel()

			hated := fullSchema()
			hated.ConflictsWith = []directive.Name{"weaver:weave"}
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "indexer:index btree", 1),
				negated(t, "weaver:weave", 2),
			}, hated, negatable("weaver", "weave"))
			assert.Length(t, got, 2, "both instances are returned")
			assert.False(t, sink.Failed(), "nothing is reported")
		})
	})
}

// Validation allocates what its instances keep and nothing to check
// them. The check runs alone, because the count includes every
// goroutine's allocations.
func TestValidateAllocs(t *testing.T) {
	r, keys, raws := validationBench(t)
	sink := diag.NewSink()
	var got []directive.Directive
	assert.MaxAllocs(t, func() { got = directive.Validate(validationSubject, raws, r, keys, nil, sink) },
		validateAllocs, "Validate allocates the instances it returns")
	assert.Length(t, got, len(raws), "every instance validates")
	assert.False(t, sink.Failed(), "nothing is reported")
}

// BenchmarkValidate measures the validation of one subject's three
// instances, which runs once per subject at the seal, so its cost per
// subject bounds the pass over a large workspace.
func BenchmarkValidate(b *testing.B) {
	b.Run("Validate", func(b *testing.B) {
		r, keys, raws := validationBench(b)
		sink := diag.NewSink()
		c := bench.Start(b).MaxAllocs(validateAllocs)
		defer c.End()
		var got []directive.Directive
		for c.Loop() {
			got = directive.Validate(validationSubject, raws, r, keys, nil, sink)
		}
		assert.Length(b, got, len(raws), "every instance validates")
		assert.False(b, sink.Failed(), "nothing is reported")
	})
}

// validationBench returns the registries and the instances the
// validation benchmark and its allocation check validate.
func validationBench(tb assert.TB) (*directive.Registry, *meta.Registry, []directive.Raw) {
	tb.Helper()

	raws := []directive.Raw{
		parse(tb, fullIndexPayload, 1),
		parse(tb, otherIndexPayload, 2),
		parse(tb, dropPayload, 3),
	}
	return sealed(tb, fullSchema(), wellFormed("stubgen", "index")), keyed(tb), raws
}

// keyed returns a metadata registry with shape.role in the group
// shape.writer, for drop resolution.
func keyed(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace("shape"), "the namespace is claimed")
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

// negatable returns a well-formed schema that admits the negated
// form.
func negatable(plugin string, name directive.Name) directive.Schema {
	s := wellFormed(plugin, name)
	s.Negatable = true
	return s
}

// parse returns the parsed payload, failing the test on a payload
// outside the grammar.
func parse(tb assert.TB, payload string, line int) directive.Raw {
	tb.Helper()

	raw, err := directive.Parse(payload)
	assert.NoError(tb, err, "the fixture payload parses")
	raw.Pos = position.Pos{File: "svc/store.go", Line: line, Col: 1}
	return raw
}

// negated returns the parsed payload in the negated form.
func negated(tb assert.TB, payload string, line int) directive.Raw {
	tb.Helper()

	raw := parse(tb, payload, line)
	raw.Negated = true
	return raw
}

// shaped returns the parsed payload marked as written on a carrier
// line in the tool-directive shape.
func shaped(tb assert.TB, payload string, line int) directive.Raw {
	tb.Helper()

	raw := parse(tb, payload, line)
	raw.DirectiveShaped = true
	return raw
}

// mixedIndexes validates two instances of the repeatable index
// schema, the first in another shape and the second in the
// tool-directive shape.
func mixedIndexes(tb assert.TB) ([]directive.Directive, *diag.Sink) {
	tb.Helper()

	return validateRaws(tb, []directive.Raw{
		parse(tb, "indexer:index hash", 1),
		shaped(tb, "indexer:index btree", 2),
	}, fullSchema())
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
	return directive.Validate(validationSubject, raws, r, keyed(tb), nil, sink), sink
}

// validateRaws runs Validate over raws against a registry of the
// kernel schemas and the given ones.
func validateRaws(
	tb assert.TB, raws []directive.Raw, schemas ...directive.Schema,
) ([]directive.Directive, *diag.Sink) {
	tb.Helper()

	sink := diag.NewSink()
	got := directive.Validate(validationSubject, raws, sealed(tb, schemas...), keyed(tb), nil, sink)
	return got, sink
}

// related returns the related positions of every finding of a sink, in
// report order.
func related(sink *diag.Sink) []position.Pos {
	var out []position.Pos
	for d := range sink.All() {
		out = append(out, d.Related...)
	}
	return out
}

// onlyDiag returns the one finding a sink contains.
func onlyDiag(tb assert.TB, sink *diag.Sink) diag.Diag {
	tb.Helper()

	found := slices.Collect(sink.All())
	assert.Length(tb, found, 1, "exactly one finding is reported")
	return found[0]
}
