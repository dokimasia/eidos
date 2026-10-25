// Copyright Dokimasia B.V. 2026
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
	"go.dokimi.dev/assert/prop"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// errMissingType is the error of the resolver of a case where no type of
// the name is in scope.
var errMissingType = errors.New("directive_test: no type of the name is in scope")

// validationSubject is the struct Store, the subject of every validation
// case.
var validationSubject = symbol.Identity{
	Lang: "golang", Package: "svc/store", Name: "Store", Kind: symbol.KindStruct,
}

// The instances of the validation benchmark. fullIndexPayload is an index
// with a positional, a list and three params under a role.
// otherIndexPayload is the index of a second plugin, dropPayload drops a
// metadata key, and writerPayload selects a variant with a list.
const (
	fullIndexPayload  = `indexer:index btree fields=[a, b] depth=3 unique=true role=server shard=id`
	otherIndexPayload = "stubgen:index mode=fast"
	dropPayload       = "meta drop=shape.role"
	writerPayload     = "shaper:shape writer reads=[a, b]"
	// derivedKey is a key of the fixture whose value its registrant
	// derives, so a meta directive cannot drop it.
	derivedKey = "shape.shape"
	// flagKey, countKey, tagsKey, targetKey and twinsKey are keys of the
	// fixture of the boolean, integer, list, identity and identity list
	// types. interfaceKey admits an interface alone, and the subject of the
	// fixture is a struct.
	flagKey      = "shape.flag"
	countKey     = "shape.count"
	tagsKey      = "shape.tags"
	targetKey    = "shape.target"
	twinsKey     = "shape.twins"
	interfaceKey = "shape.contract"
	// modeKey is a string key of the fixture that takes the choices eager
	// and lazy alone.
	modeKey = "shape.mode"
)

// The variants of the variant fixture: writer, which requires the fields
// it reads, and tx, which demands a role.
const (
	writerVariant = "writer"
	txVariant     = "tx"
)

// unregisteredCode is a code in the kernel's prefix that no package
// registers.
const unregisteredCode = "EID-9999"

// The properties of the kind check of Validate.
const (
	admitsListedKinds = "Validate must return a positive instance on a declaration of any kind " +
		"that the schema lists"
	refusesUnlistedKinds = "Validate must refuse a positive instance on a declaration of any kind " +
		"that the schema leaves out, with MisplacedDirective alone"
)

// validateAllocs is the number of allocations of a validation of the four
// instances of the benchmark. The returned slice is one allocation. Each
// instance allocates its params map, the one group of the map, and each
// param's value, which the map keeps apart because a Value is larger than
// 128 bytes. The index also allocates its positional and its list, and the
// variant allocates its list.
const validateAllocs = 1 + (2 + 4 + 2) + (2 + 1) + (2 + 1) + (2 + 1 + 1)

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
				name:     "reports UnknownKey with the keys that the schema accepts",
				payloads: []string{"indexer:index btree nonkey=v"},
				want:     directive.UnknownKey,
				naming:   "write one of fields, depth, unique, shard, role",
			},
			{
				name:     "reports ExtraPositional with the advice to write a param as key=value",
				payloads: []string{"indexer:index btree extra"},
				want:     directive.ExtraPositional,
				naming:   "write a param as key=value",
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
				name:     "reports UnknownKey with the keys that the role of the instance admits",
				payloads: []string{"indexer:index btree role=client shard=id"},
				want:     directive.UnknownKey,
				naming:   `does not accept shard under role "client": write one of fields, depth, unique, role`,
			},
			{
				name:     "reports UnknownKey for a role-scoped param on an instance without a role",
				payloads: []string{"indexer:index btree shard=id"},
				want:     directive.UnknownKey,
				naming:   "shard",
			},
			{
				name:     "reports UnknownKey with the keys that an instance without a role accepts",
				payloads: []string{"indexer:index btree shard=id"},
				want:     directive.UnknownKey,
				naming:   "does not accept shard without a role: write one of fields, depth, unique, role",
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
				name:     "reports UnknownKey with the keys of a schema that declares no roles",
				payloads: []string{"stubgen:index role=client"},
				want:     directive.UnknownKey,
				naming:   "stubgen:index declares no roles, so it does not accept role: write one of mode",
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
				name:     "reports DerivedMetadataKey for a drop of a derived key",
				payloads: []string{"meta drop=" + derivedKey},
				want:     directive.DerivedMetadataKey,
				naming:   derivedKey,
			},
			{
				name:     "reports UnclaimedName naming the nearest registered name",
				payloads: []string{"indexer:indx btree"},
				want:     directive.UnclaimedName,
				naming:   "The nearest registered name is indexer:index",
			},
			{
				name:     "reports UnclaimedName naming the nearest bare name of a bare spelling",
				payloads: []string{"shap writer"},
				want:     directive.UnclaimedName,
				naming:   "The nearest registered name is shape",
			},
			{
				name:     "reports MissingParam for a meta instance that neither drops nor sets",
				payloads: []string{"meta"},
				want:     directive.MissingParam,
				naming:   "exactly one of drop and set",
			},
			{
				name:     "reports MissingParam for a meta instance that drops and sets",
				payloads: []string{"meta drop=shape.role set=shape.role value=x"},
				want:     directive.MissingParam,
				naming:   "exactly one of drop and set",
			},
			{
				name:     "reports UnknownKey for a value beside a drop",
				payloads: []string{"meta drop=shape.role value=x"},
				want:     directive.UnknownKey,
				naming:   "meta drops shape.role and takes no value or values",
			},
			{
				name:     "reports UnknownKey with the advice to write the drop alone",
				payloads: []string{"meta drop=shape.role values=[x]"},
				want:     directive.UnknownKey,
				naming:   "write drop=shape.role alone",
			},
			{
				name:     "reports UnknownMetadataKey for a set of a group",
				payloads: []string{"meta set=shape.writer value=x"},
				want:     directive.UnknownMetadataKey,
				naming:   "a set names a key",
			},
			{
				name:     "reports DerivedMetadataKey for a set of a derived key",
				payloads: []string{"meta set=" + derivedKey + " value=x"},
				want:     directive.DerivedMetadataKey,
				naming:   "cannot drop or set it",
			},
			{
				name:     "reports MisplacedDirective for a set of a key that does not admit the kind of the subject",
				payloads: []string{"meta set=" + interfaceKey + " value=x"},
				want:     directive.MisplacedDirective,
				naming:   interfaceKey,
			},
			{
				name:     "reports BadSpelling for a set of a boolean key to false",
				payloads: []string{"meta set=" + flagKey + " value=false"},
				want:     directive.BadSpelling,
				naming:   "absence is the negative",
			},
			{
				name:     "reports BadSpelling for a set of a boolean key to another word",
				payloads: []string{"meta set=" + flagKey + " value=yes"},
				want:     directive.BadSpelling,
				naming:   `meta sets the boolean shape.flag, which takes true, not "yes"`,
			},
			{
				name:     "reports BadSpelling for a set of an integer key to text",
				payloads: []string{"meta set=" + countKey + " value=many"},
				want:     directive.BadSpelling,
				naming:   "base-10 integer",
			},
			{
				name:     "reports BadSpelling with the choices for a set of a key outside them",
				payloads: []string{"meta set=" + modeKey + " value=fast"},
				want:     directive.BadSpelling,
				naming:   `meta sets shape.mode, which takes one of eager, lazy, not "fast"`,
			},
			{
				name:     "reports TypeMismatch for a set of a list key through value",
				payloads: []string{"meta set=" + tagsKey + " value=a"},
				want:     directive.TypeMismatch,
				naming:   "through values and not value",
			},
			{
				name:     "reports TypeMismatch for a set of a scalar key through values",
				payloads: []string{"meta set=shape.role values=[a]"},
				want:     directive.TypeMismatch,
				naming:   "through value and not values",
			},
			{
				name:     "reports MissingParam for a set without a value",
				payloads: []string{"meta set=shape.role"},
				want:     directive.MissingParam,
				naming:   "omits value",
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
			{
				name:     "reports UnknownVariant naming the variants for an instance without a variant",
				payloads: []string{"shaper:shape id=a"},
				want:     directive.UnknownVariant,
				naming:   "writer, tx, getter",
			},
			{
				name:     "reports UnknownVariant naming the variants for a name that no variant has",
				payloads: []string{"shaper:shape wrtier"},
				want:     directive.UnknownVariant,
				naming:   "writer, tx, getter",
			},
			{
				name:     "reports TypeMismatch for a variant written as a list",
				payloads: []string{"shaper:shape [writer]"},
				want:     directive.TypeMismatch,
				naming:   "variant",
			},
			{
				name:     "reports MissingParam for an omitted param that the variant requires",
				payloads: []string{"shaper:shape writer"},
				want:     directive.MissingParam,
				naming:   "reads",
			},
			{
				name:     "reports UnknownKey for a param of another variant",
				payloads: []string{"shaper:shape tx role=begin reads=[a]"},
				want:     directive.UnknownKey,
				naming:   "reads",
			},
			{
				name:     "reports MissingRole naming the roles of a variant that demands one",
				payloads: []string{"shaper:shape tx"},
				want:     directive.MissingRole,
				naming:   "begin, commit",
			},
			{
				name:     "reports UnknownRole naming the roles of the variant",
				payloads: []string{"shaper:shape tx role=end"},
				want:     directive.UnknownRole,
				naming:   "begin, commit",
			},
			{
				name:     "reports UnknownKey for a role on a variant that declares none",
				payloads: []string{"shaper:shape writer reads=[a] role=begin"},
				want:     directive.UnknownKey,
				naming:   "role",
			},
			{
				name:     "reports UnknownKey with the keys of a variant that declares no roles",
				payloads: []string{"shaper:shape writer reads=[a] role=begin"},
				want:     directive.UnknownKey,
				naming:   "shaper:shape writer declares no roles, so it does not accept role: write one of id, reads",
			},
			{
				name:     "reports ExtraPositional for a positional after the variant",
				payloads: []string{"shaper:shape writer reads=[a] extra"},
				want:     directive.ExtraPositional,
				naming:   "extra",
			},
			{
				name:     "reports ExtraPositional with the advice to write a second variant in its own directive",
				payloads: []string{"shaper:shape writer reads=[a] tx"},
				want:     directive.ExtraPositional,
				naming:   "a second variant in a directive of its own",
			},
			{
				name:     "reports UnknownKey with the keys of the selected variant",
				payloads: []string{"shaper:shape tx role=begin reads=[a]"},
				want:     directive.UnknownKey,
				naming:   "write one of id, role",
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

		t.Run("returns an instance that overrides for an override schema", func(t *testing.T) {
			t.Parallel()

			overriding := wellFormed("mockgen", "stub")
			overriding.Overrides = true
			got, _ := validateRaws(t, []directive.Raw{parse(t, "mockgen:stub", 1)}, overriding)
			assert.Length(t, got, 1, "one instance is returned")
			assert.True(t, got[0].Overrides, "the instance has the override of its schema")
		})

		t.Run("returns an instance that does not override for a schema that does not", func(t *testing.T) {
			t.Parallel()

			got, _ := validateRaws(t, []directive.Raw{parse(t, "mockgen:stub", 1)}, wellFormed("mockgen", "stub"))
			assert.Length(t, got, 1, "one instance is returned")
			assert.False(t, got[0].Overrides, "the instance has no override")
		})

		t.Run("returns the variant that an instance selects", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t, "shaper:shape writer reads=[a]")
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "one instance is returned")
			assert.Equal(t, got[0].Variant, writerVariant, "the instance has the variant of its first positional")
		})

		t.Run("returns the variant that a positional after a keyed argument selects", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t, "shaper:shape reads=[a] writer")
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "one instance is returned")
			assert.Equal(t, got[0].Variant, writerVariant, "the first positional is the variant wherever it is")
		})

		t.Run("types the params of the selected variant", func(t *testing.T) {
			t.Parallel()

			got, _ := validate(t, "shaper:shape writer reads=[a]")
			assert.Length(t, got, 1, "one instance is returned")
			reads, _ := got[0].Param("reads")
			assert.Equal(t, reads, directive.Value{
				Kind: directive.TypeList, List: []directive.Value{{Kind: directive.TypeString, Str: "a"}},
			}, "the variant's param types as the variant declares")
		})

		t.Run("types the params of the schema on an instance of a variant", func(t *testing.T) {
			t.Parallel()

			got, _ := validate(t, "shaper:shape tx role=begin id=a")
			assert.Length(t, got, 1, "one instance is returned")
			id, _ := got[0].Param("id")
			assert.Equal(t, id, directive.Value{Kind: directive.TypeString, Str: "a"},
				"the schema's param applies to every variant")
		})

		t.Run("types the role of the selected variant", func(t *testing.T) {
			t.Parallel()

			got, _ := validate(t, "shaper:shape tx role=commit")
			assert.Length(t, got, 1, "one instance is returned")
			assert.Equal(t, got[0].Role, "commit", "the role is one of the variant's")
		})

		t.Run("returns the positional arguments after the variant", func(t *testing.T) {
			t.Parallel()

			targeted := variantSchema()
			targeted.Positional = []directive.ParamSpec{
				{Key: "target", Type: directive.TypeString, Doc: "the classified target"},
			}
			got, _ := validateRaws(t, []directive.Raw{parse(t, "shaper:shape writer reads=[a] main", 1)}, targeted)
			assert.Length(t, got, 1, "one instance is returned")
			assert.Equal(t, got[0].Args, []directive.Value{{Kind: directive.TypeString, Str: "main"}},
				"the variant is not among the positional arguments")
		})

		t.Run("returns one instance of each variant of a repeatable schema", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t,
				"shaper:shape writer reads=[a]",
				"shaper:shape tx role=begin")
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 2, "both instances are returned")
			expect.Equal(t, got[0].Variant, writerVariant, "the first by position is the writer")
			expect.Equal(t, got[1].Instance, 1, "the instances are numbered across the variants")
		})

		t.Run("reports DuplicateInstance for one variant of a repeatable schema written twice", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t,
				"shaper:shape writer reads=[a]",
				"shaper:shape writer reads=[b]")
			assert.Empty(t, got, "neither instance is returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.DuplicateInstance},
				"DuplicateInstance is reported")
			assert.Equal(t, related(sink), []position.Pos{{File: "svc/store.go", Line: 1, Col: 1}},
				"the finding names the first instance of the variant")
		})

		t.Run("returns the other variants beside a variant written twice", func(t *testing.T) {
			t.Parallel()

			got, _ := validate(t,
				"shaper:shape writer reads=[a]",
				"shaper:shape tx role=begin",
				"shaper:shape writer reads=[b]")
			assert.Length(t, got, 1, "the instance of the other variant is returned")
			assert.Equal(t, got[0].Variant, txVariant, "the returned instance is the transaction")
		})

		t.Run("reports DuplicateInstance for two variants of a single-instance schema", func(t *testing.T) {
			t.Parallel()

			single := variantSchema()
			single.Repeatable = false
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "shaper:shape writer reads=[a]", 1),
				parse(t, "shaper:shape tx role=begin", 2),
			}, single)
			assert.Empty(t, got, "neither instance is returned")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.DuplicateInstance},
				"DuplicateInstance is reported")
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

		sets := []struct {
			name    string
			give    string
			wantKey directive.ParamKey
			want    directive.Value
		}{
			{
				name: "types the value of a set of a string key as its text", give: "meta set=shape.role value=writer",
				wantKey: directive.MetaValue, want: directive.Value{Kind: directive.TypeString, Str: "writer"},
			},
			{
				name: "types the value of a set of a boolean key as true", give: "meta set=" + flagKey + " value=true",
				wantKey: directive.MetaValue, want: directive.Value{Kind: directive.TypeBool, Bool: true},
			},
			{
				name:    "types the value of a set of an integer key as its integer",
				give:    "meta set=" + countKey + " value=42",
				wantKey: directive.MetaValue,
				want:    directive.Value{Kind: directive.TypeInt, Int: 42},
			},
			{
				name:    "types the value of a set among the choices of its key as its text",
				give:    "meta set=" + modeKey + " value=lazy",
				wantKey: directive.MetaValue, want: directive.Value{Kind: directive.TypeString, Str: "lazy"},
			},
			{
				name:    "keeps the spelling of a set of an identity key without a resolver",
				give:    "meta set=" + targetKey + " value=Row",
				wantKey: directive.MetaValue, want: directive.Value{Kind: directive.TypeReference, Ref: "Row"},
			},
			{
				name:    "types the values of a set of a list key as a list",
				give:    "meta set=" + tagsKey + " values=[a, b]",
				wantKey: directive.MetaValues,
				want: directive.Value{Kind: directive.TypeList, List: []directive.Value{
					{Kind: directive.TypeString, Str: "a"}, {Kind: directive.TypeString, Str: "b"},
				}},
			},
			{
				name:    "keeps the spellings of a set of an identity list key without a resolver",
				give:    "meta set=" + twinsKey + " values=[Row, Col]",
				wantKey: directive.MetaValues,
				want: directive.Value{Kind: directive.TypeList, List: []directive.Value{
					{Kind: directive.TypeReference, Ref: "Row"}, {Kind: directive.TypeReference, Ref: "Col"},
				}},
			},
		}
		for _, tt := range sets {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, sink := validate(t, tt.give)
				assert.False(t, sink.Failed(), "nothing is reported")
				assert.Length(t, got, 1, "the instance is returned")
				value, held := got[0].Param(tt.wantKey)
				assert.True(t, held, "the instance has its value")
				assert.Equal(t, value, tt.want, "validation types the value as the type of the key")
			})
		}

		t.Run("binds the identity of a set through the resolver", func(t *testing.T) {
			t.Parallel()

			row := symbol.Identity{Lang: "golang", Package: "svc/store", Name: "Row", Kind: symbol.KindStruct}
			resolve := func(symbol.Identity, string, directive.ResolutionKind, string) (symbol.Identity, error) {
				return row, nil
			}
			sink := diag.NewSink()
			got := directive.Validate(
				validationSubject,
				[]directive.Raw{parse(t, "meta set="+targetKey+" value=Row", 1)},
				sealed(t),
				keyed(t),
				resolve,
				sink,
			)
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the instance is returned")
			value, _ := got[0].Param(directive.MetaValue)
			assert.Equal(t, value.Target, row, "the value is the identity that the resolver binds")
		})

		t.Run("asks the resolver without a plugin for the identity of a set", func(t *testing.T) {
			t.Parallel()

			asked := "unasked"
			resolve := func(_ symbol.Identity, name string, _ directive.ResolutionKind, owner string) (symbol.Identity, error) {
				asked = owner
				return symbol.Identity{Lang: "golang", Package: "svc/store", Name: name, Kind: symbol.KindStruct}, nil
			}
			sink := diag.NewSink()
			raws := []directive.Raw{parse(t, "meta set="+targetKey+" value=Row", 1)}
			got := directive.Validate(validationSubject, raws, sealed(t), keyed(t), resolve, sink)
			assert.Length(t, got, 1, "the instance is returned")
			assert.Equal(t, asked, "", "the kernel registers the schema of meta")
		})

		t.Run("reports UnresolvedReference for the identity of a set that does not resolve", func(t *testing.T) {
			t.Parallel()

			resolve := func(symbol.Identity, string, directive.ResolutionKind, string) (symbol.Identity, error) {
				return symbol.Identity{}, errMissingType
			}
			sink := diag.NewSink()
			got := directive.Validate(
				validationSubject,
				[]directive.Raw{parse(t, "meta set="+targetKey+" value=Ghost", 1)},
				sealed(t),
				keyed(t),
				resolve,
				sink,
			)
			assert.Empty(t, got, "the instance drops")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.UnresolvedReference},
				"the set reports the reference that does not resolve")
		})

		t.Run("binds each identity of a set of an identity list key through the resolver", func(t *testing.T) {
			t.Parallel()

			row := symbol.Identity{Lang: "golang", Package: "svc/store", Name: "Row", Kind: symbol.KindStruct}
			col := symbol.Identity{Lang: "golang", Package: "svc/store", Name: "Col", Kind: symbol.KindStruct}
			resolve := func(_ symbol.Identity, name string, _ directive.ResolutionKind, _ string) (symbol.Identity, error) {
				if name == row.Name {
					return row, nil
				}
				return col, nil
			}
			sink := diag.NewSink()
			got := directive.Validate(
				validationSubject,
				[]directive.Raw{parse(t, "meta set="+twinsKey+" values=[Row, Col]", 1)},
				sealed(t),
				keyed(t),
				resolve,
				sink,
			)
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the instance is returned")
			values, _ := got[0].Param(directive.MetaValues)
			assert.Length(t, values.List, 2, "the list has both elements")
			expect.Equal(t, values.List[0].Target, row, "the first element is the identity of Row")
			expect.Equal(t, values.List[1].Target, col, "the second element is the identity of Col")
		})

		t.Run("reports UnresolvedReference for an unresolved element of an identity list", func(t *testing.T) {
			t.Parallel()

			resolve := func(symbol.Identity, string, directive.ResolutionKind, string) (symbol.Identity, error) {
				return symbol.Identity{}, errMissingType
			}
			sink := diag.NewSink()
			got := directive.Validate(
				validationSubject,
				[]directive.Raw{parse(t, "meta set="+twinsKey+" values=[Ghost]", 1)},
				sealed(t),
				keyed(t),
				resolve,
				sink,
			)
			assert.Empty(t, got, "the instance drops")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.UnresolvedReference},
				"the set reports the element that does not resolve")
		})

		t.Run("reports TypeMismatch for the value of a set of an identity list key", func(t *testing.T) {
			t.Parallel()

			got, sink := validate(t, "meta set="+twinsKey+" value=Row")
			assert.Empty(t, got, "the instance drops")
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.TypeMismatch},
				"a key of a list takes values and not value")
		})

		tuner := directive.Schema{
			Plugin: "tuner", Name: "tune", Roles: []string{"fast", "slow"},
			Positional: []directive.ParamSpec{{Key: "level", Type: directive.TypeInt, Default: "3", Doc: "the level"}},
			Params: []directive.ParamSpec{
				{
					Key:     "mode",
					Type:    directive.TypeString,
					Choices: []string{"eager", "lazy"},
					Default: "lazy",
					Doc:     "the mode",
				},
				{Key: "depth", Type: directive.TypeInt, Default: "1", Roles: []string{"fast"}, Doc: "the depth"},
			},
			Doc: "tunes a declaration",
		}

		t.Run("applies the default of a keyed param that the instance omits", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "tuner:tune", 1)}, tuner)
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the instance is returned")
			mode, held := got[0].Param("mode")
			assert.True(t, held, "the omitted param has its default")
			assert.Equal(
				t,
				mode,
				directive.Value{Kind: directive.TypeString, Str: "lazy"},
				"the default types as the param",
			)
		})

		t.Run("keeps the written value of a param that has a default", func(t *testing.T) {
			t.Parallel()

			got, _ := validateRaws(t, []directive.Raw{parse(t, "tuner:tune mode=eager", 1)}, tuner)
			assert.Length(t, got, 1, "the instance is returned")
			mode, _ := got[0].Param("mode")
			assert.Equal(t, mode.Str, "eager", "the author's value replaces the default")
		})

		t.Run("applies the default of a positional param that the instance omits", func(t *testing.T) {
			t.Parallel()

			got, _ := validateRaws(t, []directive.Raw{parse(t, "tuner:tune", 1)}, tuner)
			assert.Length(t, got, 1, "the instance is returned")
			assert.Equal(t, got[0].Args, []directive.Value{{Kind: directive.TypeInt, Int: 3}},
				"the positional default fills the omitted position")
		})

		t.Run("applies no default under a role that does not admit the param", func(t *testing.T) {
			t.Parallel()

			got, _ := validateRaws(t, []directive.Raw{parse(t, "tuner:tune role=slow", 1)}, tuner)
			assert.Length(t, got, 1, "the instance is returned")
			_, held := got[0].Param("depth")
			assert.False(t, held, "the role slow does not admit the param")
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
			{
				name:    "reports DeprecatedDirective with the rewrite for a deprecated variant",
				payload: "shaper:shape getter",
				naming:  variantSchema().Variants[2].Deprecated,
			},
		}
		for _, tt := range deprecations {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := validateRaws(t, []directive.Raw{parse(t, tt.payload, 1)}, retired, sizing, variantSchema())
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

		t.Run("reports UnknownKey with the advice for a schema without params", func(t *testing.T) {
			t.Parallel()

			bare := directive.Schema{Plugin: "closed", Name: "mark", Doc: "marks a subject"}
			_, sink := validateRaws(t, []directive.Raw{parse(t, "closed:mark T=1", 1)}, bare)
			got := onlyDiag(t, sink)
			expect.Equal(t, got.Code, directive.UnknownKey, "the finding is under its code")
			expect.That(t, got.Msg).
				Contains("it takes no keyed param", "and states that the schema takes no keyed param")
		})

		field := func(name string) symbol.Identity {
			return symbol.Identity{Lang: "golang", Package: "svc/store", Name: name, Kind: symbol.KindField}
		}

		t.Run("binds a source reference through the resolver", func(t *testing.T) {
			t.Parallel()

			var askedKind directive.ResolutionKind
			var askedSubject symbol.Identity
			resolve := func(subject symbol.Identity, name string, kind directive.ResolutionKind, _ string) (symbol.Identity, error) {
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

		t.Run("asks the resolver with the plugin that registered the schema", func(t *testing.T) {
			t.Parallel()

			var asked string
			resolve := func(_ symbol.Identity, name string, _ directive.ResolutionKind, owner string) (symbol.Identity, error) {
				asked = owner
				return field(name), nil
			}
			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "indexer:index btree fields=[id]", 1)},
				sealed(t, fullSchema()), keyed(t), resolve, sink)
			assert.Length(t, got, 1, "the instance is returned")
			assert.Equal(t, asked, "indexer", "the resolver reads the plugin of the directive")
		})

		t.Run("reports UnresolvedReference with the resolver's reason", func(t *testing.T) {
			t.Parallel()

			resolve := func(_ symbol.Identity, name string, _ directive.ResolutionKind, _ string) (symbol.Identity, error) {
				return symbol.Identity{}, errors.New("no field named " + name)
			}
			sink := diag.NewSink()
			got := directive.Validate(validationSubject,
				[]directive.Raw{parse(t, "indexer:index btree fields=[ghost]", 1)},
				sealed(t, fullSchema()), keyed(t), resolve, sink)
			assert.Empty(t, got, "the instance is not returned")
			f := onlyDiag(t, sink)
			assert.Equal(t, f.Code, directive.UnresolvedReference, "UnresolvedReference is reported")
			assert.Contains(t, f.Msg, directive.ResolveValueField.String(), "the finding names the kind")
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

			resolve := func(symbol.Identity, string, directive.ResolutionKind, string) (symbol.Identity, error) {
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

		t.Run("returns a positive instance on any kind that the schema lists", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, admitsListedKinds, func(c *prop.Case) {
				order := c.Draw(prop.Permutation(declarationKinds()...), "the order of the kinds")
				listed := order[:c.Draw(prop.Integer(1, len(order)), "the number of listed kinds")]
				subject := validationSubject
				subject.Kind = c.Draw(prop.SampledFrom(listed...), "the kind of the subject")
				sink := diag.NewSink()
				got := directive.Validate(subject, []directive.Raw{parse(c, "mockgen:mock", 1)},
					sealed(c, kinded(listed...)), keyed(c), nil, sink)
				assert.Length(c, got, 1, "the instance is returned")
				assert.False(c, sink.Failed(), "nothing is reported")
			})
		})

		t.Run("reports MisplacedDirective for a positive instance on any kind that the schema leaves out",
			func(t *testing.T) {
				t.Parallel()

				prop.ForAll(t, refusesUnlistedKinds, func(c *prop.Case) {
					order := c.Draw(prop.Permutation(declarationKinds()...), "the order of the kinds")
					listed := order[:c.Draw(prop.Integer(1, len(order)-1), "the number of listed kinds")]
					subject := validationSubject
					subject.Kind = c.Draw(prop.SampledFrom(order[len(listed):]...), "the kind of the subject")
					sink := diag.NewSink()
					got := directive.Validate(subject, []directive.Raw{parse(c, "mockgen:mock", 1)},
						sealed(c, kinded(listed...)), keyed(c), nil, sink)
					assert.Empty(c, got, "the instance is not returned")
					assert.Equal(c, coretest.Codes(sink), []diag.Code{directive.MisplacedDirective},
						"MisplacedDirective is reported")
				})
			})

		t.Run("reports MisplacedDirective with the admitted kinds in the order of symbol.Kind", func(t *testing.T) {
			t.Parallel()

			_, sink := validateRaws(t, []directive.Raw{parse(t, "mockgen:mock", 1)},
				kinded(symbol.KindMethod, symbol.KindFunction))
			assert.Equal(t, onlyDiag(t, sink).Msg,
				"mockgen:mock applies to a declaration of kind "+symbol.KindFunction.String()+" or "+
					symbol.KindMethod.String()+", and "+validationSubject.String()+" has the kind "+
					symbol.KindStruct.String(),
				"the message states the admitted kinds and the kind of the subject")
		})

		t.Run("reports MisplacedDirective as an Error", func(t *testing.T) {
			t.Parallel()

			_, sink := validateRaws(t, []directive.Raw{parse(t, "mockgen:mock", 1)}, kinded(symbol.KindInterface))
			assert.Equal(t, onlyDiag(t, sink).Severity, diag.SeverityError, "a misplaced instance fails the run")
		})

		t.Run("reports MisplacedDirective alone for a misplaced instance that other checks refuse", func(t *testing.T) {
			t.Parallel()

			retiredMock := kinded(symbol.KindInterface)
			retiredMock.Deprecated = "write mockgen:fake in place of mockgen:mock"
			_, sink := validateRaws(t, []directive.Raw{parse(t, "mockgen:mock nonkey=v", 1)}, retiredMock)
			assert.Equal(t, coretest.Codes(sink), []diag.Code{directive.MisplacedDirective},
				"neither the unknown key nor the deprecation is reported")
		})

		t.Run("returns a negated instance on a kind that the schema leaves out", func(t *testing.T) {
			t.Parallel()

			negatableMock := kinded(symbol.KindInterface)
			negatableMock.Negatable = true
			got, sink := validateRaws(t, []directive.Raw{negated(t, "mockgen:mock", 1)}, negatableMock)
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the negated instance is returned")
		})

		t.Run("reports RequirementUnmet for a directive that requires a misplaced one", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("weaver", "weave")
			needs.Requires = []directive.Name{"mockgen:mock"}
			got, sink := validateRaws(t, []directive.Raw{
				parse(t, "weaver:weave", 1),
				parse(t, "mockgen:mock", 2),
			}, kinded(symbol.KindInterface), needs)
			assert.Empty(t, got, "neither instance is returned")
			assert.Equal(t, coretest.Codes(sink),
				[]diag.Code{directive.MisplacedDirective, directive.RequirementUnmet},
				"the misplaced instance meets no requirement")
		})

		choosy := directive.Schema{
			Plugin: "speller", Name: "spell", Doc: "spells a type",
			Params: []directive.ParamSpec{{
				Key: "int64", Type: directive.TypeString, Choices: []string{"bigint", "string", "number"},
				Doc: "the spelling of a 64-bit integer",
			}},
		}

		t.Run("types a string among its param's choices", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "speller:spell int64=string", 1)}, choosy)
			assert.False(t, sink.Failed(), "nothing is reported")
			assert.Length(t, got, 1, "the instance is returned")
			v, held := got[0].Param("int64")
			assert.True(t, held, "the param is typed")
			assert.Equal(t, v, directive.Value{Kind: directive.TypeString, Str: "string"}, "the value is the choice")
		})

		t.Run("reports BadSpelling with the choices for a string outside them", func(t *testing.T) {
			t.Parallel()

			got, sink := validateRaws(t, []directive.Raw{parse(t, "speller:spell int64=long", 1)}, choosy)
			assert.Empty(t, got, "the instance is not returned")
			d := onlyDiag(t, sink)
			expect.Equal(t, d.Code, directive.BadSpelling, "BadSpelling is reported")
			expect.Contains(t, d.Msg, "bigint, string, number", "the message lists the choices")
			expect.Contains(t, d.Msg, `"long"`, "the message contains the value")
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
		parse(tb, writerPayload, 4),
	}
	return sealed(tb, fullSchema(), wellFormed("stubgen", "index"), variantSchema()), keyed(tb), raws
}

// keyed returns a metadata registry with shape.role in the group
// shape.writer, the derived key shape.shape, and a key of each other type
// of the vocabulary, for drop and set resolution.
func keyed(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace("shape"), "the namespace is claimed")
	_, err := meta.Register[string](r, meta.KeySpec{
		Name: "shape.role", Group: "shape.writer", Doc: "the classified role",
	})
	assert.NoError(tb, err, "the fixture key registers")
	_, err = meta.Register[string](r, meta.KeySpec{
		Name: derivedKey, Doc: "the name of the classification", Derived: true,
	})
	assert.NoError(tb, err, "the derived key registers")
	_, err = meta.Register[bool](r, meta.KeySpec{Name: flagKey, Doc: "marks a flagged declaration"})
	assert.NoError(tb, err, "the boolean key registers")
	_, err = meta.Register[int64](r, meta.KeySpec{Name: countKey, Doc: "counts the members"})
	assert.NoError(tb, err, "the integer key registers")
	_, err = meta.Register[[]string](r, meta.KeySpec{Name: tagsKey, Doc: "lists the tags"})
	assert.NoError(tb, err, "the list key registers")
	_, err = meta.Register[symbol.Identity](r, meta.KeySpec{Name: targetKey, Doc: "names the twin"})
	assert.NoError(tb, err, "the identity key registers")
	_, err = meta.Register[[]symbol.Identity](r, meta.KeySpec{Name: twinsKey, Doc: "names the twins"})
	assert.NoError(tb, err, "the identity list key registers")
	_, err = meta.Register[string](r, meta.KeySpec{
		Name: interfaceKey, Kinds: []symbol.Kind{symbol.KindInterface}, Doc: "the contract of an interface",
	})
	assert.NoError(tb, err, "the key of an interface registers")
	_, err = meta.Register[string](r, meta.KeySpec{
		Name: modeKey, Choices: []string{"eager", "lazy"}, Doc: "the mode of the evaluation",
	})
	assert.NoError(tb, err, "the key with choices registers")
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

// variantSchema returns a repeatable schema with a param for every
// variant and three variants: writer, which requires the fields it reads;
// tx, which demands one of two roles; and getter, which is deprecated.
func variantSchema() directive.Schema {
	return directive.Schema{
		Plugin: "shaper",
		Name:   "shape",
		Params: []directive.ParamSpec{
			{Key: "id", Type: directive.TypeString, Doc: "separates two instances of one protocol"},
		},
		Variants: []directive.Variant{
			{
				Name: writerVariant,
				Params: []directive.ParamSpec{
					{
						Key: "reads", Type: directive.TypeList, ListOf: directive.TypeString, Required: true,
						Doc: "the fields that the writer reads",
					},
				},
				Doc: "writes a value",
			},
			{Name: txVariant, Roles: []string{"begin", "commit"}, RolesRequired: true, Doc: "a transaction"},
			{Name: "getter", Doc: "reads a value", Deprecated: "write shaper:shape reader in place of getter"},
		},
		Repeatable: true,
		Doc:        "classifies a callable",
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

	r := sealed(tb, fullSchema(), wellFormed("stubgen", "index"), variantSchema())
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
