// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"bytes"
	"math/big"
	"net/netip"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The file of the error fixture, the types that it declares, the field
// that is the cause of NotFound, and the methods of the error contract.
const (
	errorsFile   = "fx/errors.go"
	notFoundName = "NotFound"
	timeoutName  = "Timeout"
	codeName     = "Code"
	joinedName   = "Joined"
	messageName  = "Message"
	countName    = "Count"
	faultName    = "Fault"
	legacyName   = "Legacy"
	wrapperName  = "Wrapper"
	causeField   = "err"
	errorName    = "Error"
	unwrapName   = "Unwrap"
	isName       = "Is"
	asName       = "As"
	stringName   = "String"
)

// The file of the fixture of hidden fields, its struct with a blank field,
// and the package of the stand-in of math/big.
const (
	hiddenFile = "fx/hidden.go"
	paddedName = "Padded"
	bigPath    = "math/big"
)

// The packages of the import cases: the package of the code, the package of
// the referents, and an internal package of the tree of the referents.
const (
	webPath   = "example.com/shop/web"
	corePath  = "example.com/shop/core"
	clockPath = corePath + "/internal/clock"
)

// The capability cases read the sentinel that a base spells, the struct
// tag that Tagged.Name of the fixture declares, key by key, and the
// spellings of the witness and the comparison cases.
const (
	// sentinelBase is the base the convention spells as sentinel.
	sentinelBase = "not found"
	sentinel     = "ErrNotFound"
	// jsonKey and dbKey are the keys the tag states, with their
	// values, and xmlKey a key it does not state.
	jsonKey = "json"
	jsonTag = "name,omitempty"
	dbKey   = "db"
	dbTag   = "n"
	xmlKey  = "xml"
	// witness is the type every admitting bound derives.
	witness = "int"
	// paramName is the name of a type parameter without a bound, and
	// argSpelling a builtin that compares.
	paramName   = "T"
	argSpelling = "string"
	// stampedHost is the struct whose embed the member walk cannot
	// read, stampedLabel its own field, and timeSpelling the embed.
	stampedHost  = "Stamped"
	stampedLabel = "Label"
	timeSpelling = "time.Time"
)

// The allocations of the capabilities.
const (
	// sentinelAllocs is a sentinel's name: the base in Pascal case, and
	// the prefix joined onto it.
	sentinelAllocs = 2
	// witnessAllocs is a derived witness: the reference to int.
	witnessAllocs = 1
	// settableAllocs is the settable set of a struct that embeds one
	// type: the binding the member walk runs on, the walk's member list,
	// the path into the embed, and the list of settable fields.
	settableAllocs = 4
	// comparableAllocs is Row, whose slice and map fields break
	// comparability: the problem list as it grows to the two.
	comparableAllocs = 2
	// methodSetAllocs is the method set of NotFound: the binding that the
	// member walk runs on, the member list of the walk, and the list of
	// methods. errorContractAllocs is the method set alone.
	methodSetAllocs     = 3
	errorContractAllocs = methodSetAllocs
)

// TestCapabilities pins each method of the optional capability interfaces
// of the Go rules, through the interface that declares it.
func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("SentinelName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the base in PascalCase behind Err", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capability[rules.ErrorValueRules](t).SentinelName(sentinelBase), sentinel,
				"the prefix and the base")
		})
	})

	t.Run("SentinelBase", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			give         string
			want         string
			wantSentinel bool
		}{
			{
				name: "returns the base after the prefix and an upper-case rune", give: sentinel,
				want: notFoundName, wantSentinel: true,
			},
			{name: "reports false for the prefix alone", give: "Err"},
			{name: "reports false for a lower-case rune after the prefix", give: "Errors"},
			{name: "reports false for a name without the prefix", give: notFoundName},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				base, isSentinel := capability[rules.ErrorValueRules](t).SentinelBase(tt.give)
				expect.Equal(t, isSentinel, tt.wantSentinel, "whether the name follows the convention")
				expect.Equal(t, base, tt.want, "the base of the name")
			})
		}

		t.Run("recognises the name that SentinelName returns", func(t *testing.T) {
			t.Parallel()

			errs := capability[rules.ErrorValueRules](t)
			_, isSentinel := errs.SentinelBase(errs.SentinelName(sentinelBase))
			assert.True(t, isSentinel, "a composed name round-trips through the recognition")
		})
	})

	t.Run("MethodSet", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the methods that an embed promotes", func(t *testing.T) {
			t.Parallel()

			f := errorsLoaded(t)
			set := gorules.MethodSet(f.decl(t, id(fxPath, wrapperName, symbol.KindStruct)), f.view)
			assert.Equal(t, methodNames(set), []string{errorName, unwrapName, isName, asName},
				"the methods of the embedded NotFound")
		})

		t.Run("returns the file-level methods of a defined type", func(t *testing.T) {
			t.Parallel()

			f := errorsLoaded(t)
			set := gorules.MethodSet(f.decl(t, id(fxPath, codeName, symbol.KindAlias)), f.view)
			assert.Equal(t, methodNames(set), []string{errorName}, "the method that the package attaches to Code")
		})

		t.Run("returns nil for a symbol that is not a declaration", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, gorules.MethodSet(nil, errorsLoaded(t).view), "nothing has no methods")
		})
	})

	t.Run("NullaryString", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveHost string
			giveName string
			want     bool
		}{
			{
				name:     "returns the method of the name that returns a string",
				giveHost: notFoundName, giveName: errorName, want: true,
			},
			{
				name:     "returns the method whose result is an alias of string",
				giveHost: messageName, giveName: errorName, want: true,
			},
			{
				name:     "returns nil for a method of the name that returns another type",
				giveHost: countName, giveName: errorName,
			},
			{name: "returns nil for a name that the set does not have", giveHost: notFoundName, giveName: stringName},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := errorsLoaded(t)
				set := gorules.MethodSet(f.decl(t, id(fxPath, tt.giveHost, symbol.KindStruct)), f.view)
				got := gorules.NullaryString(set, tt.giveName, f.view)
				assert.Equal(t, got != nil, tt.want, "whether the set has the method")
			})
		}
	})

	t.Run("ErrorContract", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the cause and the three hooks of a struct", func(t *testing.T) {
			t.Parallel()

			c, isError := errorContract(t, notFoundName, symbol.KindStruct)
			assert.True(t, isError, "NotFound is an error")
			expect.That(t, c.Cause).NotNil("NotFound has a cause")
			expect.Equal(t, c.Cause.Name, causeField, "the cause is the field err")
			expect.That(t, c.Unwrap).NotNil("NotFound has the hook Unwrap")
			expect.That(t, c.Is).NotNil("NotFound has the hook Is")
			expect.That(t, c.As).NotNil("NotFound has the hook As")
			expect.False(t, c.Pointer, "a value of NotFound is the error")
		})

		t.Run("returns the method Error as the message", func(t *testing.T) {
			t.Parallel()

			c, _ := errorContract(t, notFoundName, symbol.KindStruct)
			message, isMethod := c.Message.(*node.Method)
			assert.True(t, isMethod, "the message is a method")
			assert.Equal(t, message.Name, errorName, "the method Error")
		})

		t.Run("reports a pointer receiver of the method Error", func(t *testing.T) {
			t.Parallel()

			c, isError := errorContract(t, timeoutName, symbol.KindStruct)
			assert.True(t, isError, "Timeout is an error")
			assert.True(t, c.Pointer, "a pointer to Timeout is the error")
		})

		t.Run("returns no cause where Unwrap returns many errors", func(t *testing.T) {
			t.Parallel()

			c, isError := errorContract(t, joinedName, symbol.KindStruct)
			assert.True(t, isError, "Joined is an error")
			expect.That(t, c.Unwrap).NotNil("Joined has the hook Unwrap")
			expect.That(t, c.Cause).Nil("Unwrap() []error does not name one field")
		})

		t.Run("returns no cause for a field that only an embed declares", func(t *testing.T) {
			t.Parallel()

			c, isError := errorContract(t, wrapperName, symbol.KindStruct)
			assert.True(t, isError, "Wrapper is an error through its embed")
			expect.That(t, c.Unwrap).NotNil("the embed promotes the hook Unwrap")
			expect.That(t, c.Cause).Nil("Wrapper does not declare the field err")
		})

		t.Run("returns the hook As over the empty inline interface", func(t *testing.T) {
			t.Parallel()

			c, _ := errorContract(t, legacyName, symbol.KindStruct)
			assert.NotNil(t, c.As, "As(interface{}) bool is the hook As")
		})

		t.Run("returns the contract of a defined type", func(t *testing.T) {
			t.Parallel()

			_, isError := errorContract(t, codeName, symbol.KindAlias)
			assert.True(t, isError, "Code is an error")
		})

		t.Run("returns the contract of an interface", func(t *testing.T) {
			t.Parallel()

			_, isError := errorContract(t, faultName, symbol.KindInterface)
			assert.True(t, isError, "a value of Fault is an error")
		})

		t.Run("reports false for a method Error that returns another type", func(t *testing.T) {
			t.Parallel()

			_, isError := errorContract(t, countName, symbol.KindStruct)
			assert.False(t, isError, "the method Error of Count returns an int")
		})

		t.Run("reports false for a symbol that is not a declaration", func(t *testing.T) {
			t.Parallel()

			_, isError := capability[rules.ErrorTypeRules](t).ErrorContract(nil, errorsLoaded(t).view)
			assert.False(t, isError, "nothing is no error")
		})
	})

	t.Run("ValueReceiver", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveHost string
			giveName string
			want     bool
		}{
			{name: "reports true for a method with a value receiver", giveHost: "Row", giveName: "Rename", want: true},
			{name: "reports false for a method with a pointer receiver", giveHost: "Store", giveName: "Put"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				host, is := f.decl(t, id(fxPath, tt.giveHost, symbol.KindStruct)).(*node.Struct)
				assert.True(t, is, "the host is a struct")
				m, held := node.MemberNamed(host.Methods, tt.giveName)
				assert.True(t, held, "the host declares the method")
				assert.Equal(t, capability[rules.SatisfactionRules](t).ValueReceiver(m, f.view), tt.want,
					"whether the method set of a value has the method")
			})
		}
	})

	t.Run("TypeSet", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for an interface with type-set elements", give: "Number", want: true},
			{name: "reports false for an interface of methods", give: "Reader"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				i, is := f.decl(t, id(fxPath, tt.give, symbol.KindInterface)).(*node.Interface)
				assert.True(t, is, "the declaration is an interface")
				assert.Equal(t, capability[rules.SatisfactionRules](t).TypeSet(i, f.view), tt.want,
					"whether the interface constrains a type set")
			})
		}
	})

	t.Run("Tag", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			key        string
			want       string
			wantStated bool
		}{
			{name: "returns the value of the json key", key: jsonKey, want: jsonTag, wantStated: true},
			{name: "returns the value of the db key", key: dbKey, want: dbTag, wantStated: true},
			{name: "reports false for a key the tag does not state", key: xmlKey},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, stated := capability[rules.TagRules](t).Tag(f.field(t, "Tagged", "Name"), tt.key)
				assert.Equal(t, stated, tt.wantStated, "whether the tag states the key")
				assert.Equal(t, got, tt.want, "the key's value")
			})
		}

		t.Run("reports false for a missing field", func(t *testing.T) {
			t.Parallel()

			_, stated := capability[rules.TagRules](t).Tag(nil, jsonKey)
			assert.False(t, stated, "no field, no tag")
		})
	})

	t.Run("Importable", func(t *testing.T) {
		t.Parallel()

		web := id(webPath, "", symbol.KindPackage)
		clock := ref(clockPath, "Clock", symbol.KindInterface)
		admitted := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "returns nil for a nil reference"},
			{name: "returns nil for a predeclared type", give: &node.TypeRef{Spelling: "int"}},
			{
				name: "returns nil for an exported declaration of another package",
				give: ref(corePath, "Row", symbol.KindStruct),
			},
			{
				name: "returns nil for an unexported declaration of the package of the code",
				give: ref(webPath, "rows", symbol.KindStruct),
			},
			{
				name: "returns nil for a type parameter of another package",
				give: ref(corePath, "T", symbol.KindTypeParam),
			},
			{
				name: "returns nil for an exported declaration that the reference names without a target",
				give: &node.TypeRef{Spelling: "core.Row", Package: corePath},
			},
		}
		for _, tt := range admitted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, capability[rules.ImportRules](t).Importable(web, tt.give),
					"Go code of the package refers to the declaration")
			})
		}

		refused := []struct {
			name string
			give *node.TypeRef
			want string
		}{
			{
				name: "returns an error for a target of an internal tree that does not contain the code",
				give: clock, want: "internal to " + corePath,
			},
			{
				name: "returns an error for an unexported target of another package",
				give: ref(corePath, "rows", symbol.KindStruct), want: "rows is not exported",
			},
			{
				name: "returns an error for a function of an external test package",
				give: ref(corePath+golang.TestSuffix, "NewBox", symbol.KindFunction), want: "external test package",
			},
			{
				name: "returns an error for the spelling of an unexported declaration without a target",
				give: &node.TypeRef{Spelling: "core.rows", Package: corePath}, want: "rows is not exported",
			},
			{
				name: "returns an error for the element of a list that the code cannot refer to",
				give: &node.TypeRef{Form: symbol.FormList, Spelling: "[]clock.Clock", Elems: []*node.TypeRef{clock}},
				want: "internal to " + corePath,
			},
			{
				name: "returns an error for a type argument that the code cannot refer to",
				give: &node.TypeRef{
					Spelling: "Box", Target: id(webPath, "Box", symbol.KindStruct), Args: []*node.TypeRef{clock},
				},
				want: "internal to " + corePath,
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := capability[rules.ImportRules](t).Importable(web, tt.give)
				assert.HasError(t, err, "Go code of the package cannot refer to the declaration")
				assert.Contains(t, err.Error(), tt.want, "the error names the rule")
			})
		}
	})

	t.Run("ImportableValue", func(t *testing.T) {
		t.Parallel()

		web := id(webPath, "", symbol.KindPackage)
		worn := emit.Name(id(clockPath, "ErrWorn", symbol.KindVariable))
		one := emit.Literal(emit.LiteralInt, "1")
		admitted := []struct {
			name string
			give emit.Value
		}{
			{name: "returns nil for the zero value"},
			{name: "returns nil for a literal", give: one},
			{
				name: "returns nil for a call of an exported function of another package",
				give: emit.Call(id(corePath, "NewRow", symbol.KindFunction)),
			},
			{
				name: "returns nil for an unexported variable of the package of the code",
				give: emit.Name(id(webPath, "errGone", symbol.KindVariable)),
			},
			{
				name: "returns nil for a composite of an exported type of another package",
				give: emit.Composite(rules.EmitRef(ref(corePath, "Row", symbol.KindStruct)), emit.Element(one)),
			},
			{
				name: "returns nil for an exported field of a composite of another package",
				give: emit.Composite(
					rules.EmitRef(ref(corePath, "Row", symbol.KindStruct)),
					emit.NamedField("Slack", one),
				),
			},
			{
				name: "returns nil for an unexported field of a composite of the package of the code",
				give: emit.Composite(
					rules.EmitRef(ref(webPath, "grip", symbol.KindStruct)),
					emit.NamedField("slack", one),
				),
			},
		}
		for _, tt := range admitted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, capability[rules.ValueImportRules](t).ImportableValue(web, tt.give),
					"Go code of the package refers to every declaration of the value")
			})
		}

		internalTo := "internal to " + corePath
		refused := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "returns an error for a variable of an internal tree", give: worn, want: internalTo},
			{
				name: "returns an error for a call of an unexported function of another package",
				give: emit.Call(id(corePath, "newRow", symbol.KindFunction)), want: "newRow is not exported",
			},
			{
				name: "returns an error for a conversion to a type of an internal tree",
				give: emit.Conversion(rules.EmitRef(ref(clockPath, "Level", symbol.KindAlias)), one), want: internalTo,
			},
			{
				name: "returns an error for a type argument of the type of a composite",
				give: emit.Composite(rules.EmitRef(&node.TypeRef{
					Spelling: "Box", Target: id(webPath, "Box", symbol.KindStruct),
					Args: []*node.TypeRef{ref(clockPath, "Clock", symbol.KindInterface)},
				})),
				want: internalTo,
			},
			{
				name: "returns an error for the value of a field of a composite",
				give: emit.Composite(rules.EmitRef(ref(webPath, "Probe", symbol.KindStruct)),
					emit.NamedField("Err", worn)),
				want: internalTo,
			},
			{
				name: "returns an error for the key of an entry of a composite",
				give: emit.Composite(rules.EmitRef(ref(webPath, "Index", symbol.KindAlias)),
					emit.KeyedEntry(worn, one)),
				want: internalTo,
			},
			{
				name: "returns an error for an argument of a call",
				give: emit.Call(id(corePath, "Wrap", symbol.KindFunction), worn), want: internalTo,
			},
			{name: "returns an error for the inner value of an address", give: emit.Address(worn), want: internalTo},
			{
				name: "returns an error for an unexported field of a composite of another package",
				give: emit.Composite(
					rules.EmitRef(ref(corePath, "Row", symbol.KindStruct)),
					emit.NamedField("slack", one),
				),
				want: "slack is not exported",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := capability[rules.ValueImportRules](t).ImportableValue(web, tt.give)
				assert.HasError(t, err, "Go code of the package cannot refer to a declaration of the value")
				assert.Contains(t, err.Error(), tt.want, "the error names the rule")
			})
		}
	})

	t.Run("Derive", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			host  string
			param int
			want  string
		}{
			{name: "returns int for a parameter bound by any", host: "Box", want: witness},
			{name: "returns int for a parameter bound by comparable", host: "Pair", want: witness},
			{name: "returns int for a type set that names ~int", host: "Sized", want: witness},
			{name: "returns int for an empty interface the graph declares", host: "Sized", param: 2, want: witness},
			{name: "reports false for a bound that states methods", host: "Bound"},
			{name: "reports false for a type set that names no int", host: "Sized", param: 1},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				w, derived := capability[rules.GenericsRules](t).Derive(paramsOf(t, f, tt.host)[tt.param], f.view)
				assert.Equal(t, derived, tt.want != "", "whether int satisfies the bound")
				if derived {
					assert.Equal(t, w.Spelling, tt.want, "the witness")
				}
			})
		}

		t.Run("returns int for a parameter without a bound", func(t *testing.T) {
			t.Parallel()

			w, derived := capability[rules.GenericsRules](t).Derive(&node.TypeParam{Name: paramName}, rules.View{})
			assert.True(t, derived, "no bound admits every type")
			assert.Equal(t, w.Spelling, witness, "the witness")
		})

		t.Run("returns int for a constraint interface in a view without facts", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			bare := rules.NewView(f.view.Reader(), nil, nil, f.view.Kernel())
			_, derived := capability[rules.GenericsRules](t).Derive(paramsOf(t, f, "Sized")[1], bare)
			assert.True(t, derived, "a view without facts reads no type set, so the empty interface admits int")
		})

		t.Run("reports false for a missing parameter", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, derived := capability[rules.GenericsRules](t).Derive(nil, f.view)
			assert.False(t, derived, "no parameter, no witness")
		})

		t.Run("derives one witness per parameter through the bound rules", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			assert.Length(t, f.bound().Witnesses(paramsOf(t, f, "Pair")), 2, "Pair's key and value")
		})
	})

	t.Run("Reified", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false", func(t *testing.T) {
			t.Parallel()

			assert.False(t, capability[rules.GenericsRules](t).Reified(), "Go erases type arguments at run time")
		})
	})

	t.Run("Settable", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the promoted fields after the struct's own", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, settableNames(t, "Derived"), []string{"Name", "Kind"},
				"Derived's own field, then the one Base promotes")
		})

		t.Run("leaves out an unexported field", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, settableNames(t, "Row"),
				[]string{"ID", "Tags", "Next", "When", "Dur", "Dep", "Set", "Ratio"},
				"another package cannot set Row's name")
		})

		t.Run("returns the settable fields beside a gap", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, settableNames(t, stampedHost), []string{stampedLabel},
				"the struct's own exported field, whatever its embed yields")
		})

		t.Run("returns the gap of an embed the walk cannot read", func(t *testing.T) {
			t.Parallel()

			gaps := settable(t, stampedHost).Gaps
			assert.Length(t, gaps, 1, "one gap, so a builder over the set knows it is partial")
			assert.Equal(t, gaps[0].Reason, rules.GapUnresolved, "the embed resolves to nothing the view contains")
			assert.Equal(t, gaps[0].Contributor.Spelling, timeSpelling, "naming the embed")
		})

		t.Run("returns an empty set for a missing struct", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			set := capability[rules.PromotionRules](t).Settable(nil, f.view)
			assert.Empty(t, set.Members, "nothing has no members")
			assert.Empty(t, set.Gaps, "and no gaps")
		})
	})

	t.Run("Comparable", func(t *testing.T) {
		t.Parallel()

		resolvedDuration := builtin(durationSpelling)
		resolvedDuration.Target = id(timePath, "Duration", symbol.KindAlias)
		tests := []struct {
			name string
			give *node.TypeRef
			want bool
		}{
			{
				name: "reports true for a struct of strings",
				give: ref(fxPath, "Base", symbol.KindStruct), want: true,
			},
			{
				name: "reports true for a struct that embeds a comparable struct",
				give: ref(fxPath, "Derived", symbol.KindStruct), want: true,
			},
			{
				name: "reports true for an interface",
				give: ref(fxPath, "Reader", symbol.KindInterface), want: true,
			},
			{
				name: "reports true for an enumeration",
				give: ref(fxPath, "Color", symbol.KindEnum), want: true,
			},
			{
				name: "reports true for a defined type over a comparable type",
				give: ref(fxPath, "Weight", symbol.KindAlias), want: true,
			},
			{
				name: "reports true for an instantiation whose argument compares",
				give: boxOf(builtin(witness)), want: true,
			},
			{
				name: "reports true for a pointer",
				give: composite("*Row", symbol.FormOptional, builtin("Row")), want: true,
			},
			{
				name: "reports true for a channel",
				give: composite("chan int", symbol.FormStream, builtin(witness)), want: true,
			},
			{
				name: "reports true for an array of a comparable element",
				give: composite("[2]int", symbol.FormArray, builtin(witness)), want: true,
			},
			{
				name: "reports true for an empty inline interface",
				give: &node.TypeRef{Spelling: "interface{}", Form: symbol.FormInline}, want: true,
			},
			{
				name: "reports true for an inline interface that states methods",
				give: &node.TypeRef{Spelling: "interface{ Close() error }", Form: symbol.FormInline}, want: true,
			},
			{name: "reports true for any", give: builtin("any"), want: true},
			{name: "reports true for string", give: builtin(argSpelling), want: true},
			{name: "reports true for complex64", give: builtin("complex64"), want: true},
			{name: "reports true for unsafe.Pointer", give: builtin("unsafe.Pointer"), want: true},
			{
				name: "reports false for a struct with a slice field",
				give: ref(fxPath, "Uncomparable", symbol.KindStruct),
			},
			{
				name: "reports false for a struct with several uncomparable fields",
				give: ref(fxPath, "Row", symbol.KindStruct),
			},
			{
				name: "reports false for an instantiation whose argument does not compare",
				give: boxOf(composite("[]int", symbol.FormList, builtin(witness))),
			},
			{name: "reports false for a generic type without its arguments", give: boxOf(nil)},
			{name: "reports false for a function type", give: composite("func()", symbol.FormFunc)},
			{
				name: "reports false for a map",
				give: composite("map[int]int", symbol.FormMap, builtin(witness), builtin(witness)),
			},
			{
				name: "reports true for the empty inline struct",
				give: &node.TypeRef{Spelling: "struct{}", Form: symbol.FormInline}, want: true,
			},
			{
				name: "reports true for an inline struct of comparable fields",
				give: &node.TypeRef{
					Spelling: "struct{ A int }", Form: symbol.FormInline,
					Fields: []*node.Field{{Name: "A", Type: builtin(witness)}},
				},
				want: true,
			},
			{
				name: "reports false for an inline struct with a slice field",
				give: &node.TypeRef{
					Spelling: "struct{ S []int }", Form: symbol.FormInline,
					Fields: []*node.Field{{Name: "S", Type: composite("[]int", symbol.FormList, builtin(witness))}},
				},
			},
			{
				name: "reports false for an inline struct with an embedded field",
				give: &node.TypeRef{Spelling: "struct{ io.Reader }", Form: symbol.FormInline},
			},
			{
				name: "reports false for an inline struct with a blank field",
				give: &node.TypeRef{
					Spelling: "struct{ _ []int; A int }", Form: symbol.FormInline,
					Fields: []*node.Field{{Name: "A", Type: builtin(witness)}},
				},
			},
			{
				name: "reports false for an inline struct whose spelling does not parse",
				give: &node.TypeRef{Spelling: "struct{", Form: symbol.FormInline},
			},
			{
				name: "reports false for an inline type of another kind",
				give: &node.TypeRef{Spelling: "func()", Form: symbol.FormInline},
			},
			{name: "reports true for time.Time outside the workspace", give: builtin(timeSpelling), want: true},
			{name: "reports true for time.Duration outside the workspace", give: builtin(durationSpelling), want: true},
			{
				name: "reports true for time.Duration through a target outside the view",
				give: resolvedDuration, want: true,
			},
			{name: "reports false for a name outside the workspace", give: builtin("Outside")},
			{
				name: "reports false for a predeclared name another package qualifies",
				give: &node.TypeRef{Spelling: "p.int", Package: foreignPkg},
			},
			{
				name: "reports false for a target the graph does not declare",
				give: ref(fxPath, "Ghost", symbol.KindStruct),
			},
			{name: "reports false for a missing reference"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ok, _ := capability[rules.EqualityRules](t).Comparable(tt.give, f.view)
				assert.Equal(t, ok, tt.want, "whether Go compares the type with ==")
			})
		}

		t.Run("returns no problem for a type that compares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, problems := capability[rules.EqualityRules](t).Comparable(ref(fxPath, "Base", symbol.KindStruct), f.view)
			assert.Empty(t, problems, "nothing breaks comparability")
		})

		t.Run("returns the slice field as the problem", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, problems := capability[rules.EqualityRules](t).Comparable(
				ref(fxPath, "Uncomparable", symbol.KindStruct), f.view)
			assert.Length(t, problems, 1, "one field breaks comparability")
			assert.Equal(t, problems[0].Form, symbol.FormList, "the slice")
		})

		t.Run("returns the argument that does not compare as the problem", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, problems := capability[rules.EqualityRules](t).Comparable(
				boxOf(composite("[]int", symbol.FormList, builtin(witness))), f.view)
			assert.Length(t, problems, 1, "one argument breaks comparability")
			assert.Equal(t, problems[0].Form, symbol.FormList, "the slice the argument binds")
		})

		// The table of the standard library answers as Go answers, which
		// reflect reads from the type that the compiler built.
		std := []struct {
			name     string
			givePkg  string
			giveName string
			want     reflect.Type
		}{
			{
				name: "reports what Go reports for time.Time", givePkg: "time", giveName: "Time",
				want: reflect.TypeFor[time.Time](),
			},
			{
				name: "reports what Go reports for time.Location", givePkg: "time", giveName: "Location",
				want: reflect.TypeFor[time.Location](),
			},
			{
				name: "reports what Go reports for netip.Addr", givePkg: "net/netip", giveName: "Addr",
				want: reflect.TypeFor[netip.Addr](),
			},
			{
				name: "reports what Go reports for netip.AddrPort", givePkg: "net/netip", giveName: "AddrPort",
				want: reflect.TypeFor[netip.AddrPort](),
			},
			{
				name: "reports what Go reports for netip.Prefix", givePkg: "net/netip", giveName: "Prefix",
				want: reflect.TypeFor[netip.Prefix](),
			},
			{
				name: "reports what Go reports for url.URL", givePkg: "net/url", giveName: "URL",
				want: reflect.TypeFor[url.URL](),
			},
			{
				name: "reports what Go reports for url.Userinfo", givePkg: "net/url", giveName: "Userinfo",
				want: reflect.TypeFor[url.Userinfo](),
			},
			{
				name: "reports what Go reports for sync.Mutex", givePkg: "sync", giveName: "Mutex",
				want: reflect.TypeFor[sync.Mutex](),
			},
			{
				name: "reports what Go reports for sync.RWMutex", givePkg: "sync", giveName: "RWMutex",
				want: reflect.TypeFor[sync.RWMutex](),
			},
			{
				name: "reports what Go reports for sync.Once", givePkg: "sync", giveName: "Once",
				want: reflect.TypeFor[sync.Once](),
			},
			{
				name: "reports what Go reports for sync.WaitGroup", givePkg: "sync", giveName: "WaitGroup",
				want: reflect.TypeFor[sync.WaitGroup](),
			},
			{
				name: "reports what Go reports for big.Int", givePkg: bigPath, giveName: "Int",
				want: reflect.TypeFor[big.Int](),
			},
			{
				name: "reports what Go reports for big.Float", givePkg: bigPath, giveName: "Float",
				want: reflect.TypeFor[big.Float](),
			},
			{
				name: "reports what Go reports for big.Rat", givePkg: bigPath, giveName: "Rat",
				want: reflect.TypeFor[big.Rat](),
			},
			{
				name: "reports what Go reports for bytes.Buffer", givePkg: "bytes", giveName: "Buffer",
				want: reflect.TypeFor[bytes.Buffer](),
			},
			{
				name: "reports what Go reports for strings.Builder", givePkg: "strings", giveName: "Builder",
				want: reflect.TypeFor[strings.Builder](),
			},
			{
				name: "reports what Go reports for regexp.Regexp", givePkg: "regexp", giveName: "Regexp",
				want: reflect.TypeFor[regexp.Regexp](),
			},
		}
		for _, tt := range std {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				give := &node.TypeRef{Spelling: path.Base(tt.givePkg) + "." + tt.giveName, Package: tt.givePkg}
				ok, _ := capability[rules.EqualityRules](t).Comparable(give, f.view)
				assert.Equal(t, ok, tt.want.Comparable(), "the answer of Go for "+tt.want.String())
			})
		}
		for _, tt := range std {
			t.Run(tt.name+" through a target outside the view", func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				give := &node.TypeRef{
					Spelling: path.Base(tt.givePkg) + "." + tt.giveName, Package: tt.givePkg,
					Target: id(tt.givePkg, tt.giveName, symbol.KindStruct),
				}
				ok, _ := capability[rules.EqualityRules](t).Comparable(give, f.view)
				assert.Equal(t, ok, tt.want.Comparable(), "the answer of Go for a resolved "+tt.want.String())
			})
		}

		hidden := []struct {
			name string
			give symbol.Identity
			want bool
		}{
			{name: "reports false for a struct with a blank field", give: id(fxPath, paddedName, symbol.KindStruct)},
			{
				name: "reports true for a struct of the table with hidden fields that Go compares",
				give: id(timePath, "Time", symbol.KindStruct), want: true,
			},
			{
				name: "reports false for a struct of the table with hidden fields that Go does not compare",
				give: id(bigPath, "Int", symbol.KindStruct),
			},
		}
		for _, tt := range hidden {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := hiddenLoaded(t)
				ok, _ := capability[rules.EqualityRules](t).Comparable(
					&node.TypeRef{Spelling: tt.give.Name, Target: tt.give}, f.view)
				assert.Equal(t, ok, tt.want, "whether the rules prove the struct comparable")
			})
		}
	})
}

// Each capability allocates what it returns and the working state its
// walk needs. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestCapabilitiesAllocs(t *testing.T) {
	checkAllocs(t, capabilityCalls(t))
}

// BenchmarkCapabilities measures each capability the kernel calls
// through its optional interfaces.
func BenchmarkCapabilities(b *testing.B) {
	benchCalls(b, capabilityCalls(b))
}

// capabilityCalls returns a call of every capability over the fixture
// tree.
func capabilityCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f, ef := loaded(tb), errorsLoaded(tb)
	errs, tags := capability[rules.ErrorValueRules](tb), capability[rules.TagRules](tb)
	generics, promotion := capability[rules.GenericsRules](tb), capability[rules.PromotionRules](tb)
	equality, errorTypes := capability[rules.EqualityRules](tb), capability[rules.ErrorTypeRules](tb)
	imports, web := capability[rules.ImportRules](tb), id(webPath, "", symbol.KindPackage)
	coreRow := ref(corePath, "Row", symbol.KindStruct)
	values := capability[rules.ValueImportRules](tb)
	rowValue := emit.Composite(rules.EmitRef(coreRow),
		emit.NamedField("Next", emit.Call(id(corePath, "NewRow", symbol.KindFunction))))
	tagged, box := f.field(tb, "Tagged", "Name"), paramsOf(tb, f, "Box")
	derived, _ := f.decl(tb, id(fxPath, "Derived", symbol.KindStruct)).(*node.Struct)
	row, base := ref(fxPath, "Row", symbol.KindStruct), ref(fxPath, "Base", symbol.KindStruct)
	notFound := ef.decl(tb, id(fxPath, notFoundName, symbol.KindStruct))
	notFoundSet := gorules.MethodSet(notFound, ef.view)
	var (
		text     string
		reported bool
		got      *node.TypeRef
		set      rules.MemberSet
		problems []*node.TypeRef
		methods  []*node.Method
		message  *node.Method
		contract rules.ErrorContract
		refusal  error
	)
	return []allocCall{
		{
			name: "Importable",
			call: func() { refusal = imports.Importable(web, coreRow) },
			check: func(tb assert.TB) {
				assert.NoError(tb, refusal, "Importable admits an exported declaration of another package")
			},
		},
		{
			name: "ImportableValue",
			call: func() { refusal = values.ImportableValue(web, rowValue) },
			check: func(tb assert.TB) {
				assert.NoError(tb, refusal, "ImportableValue admits a composite of an exported type of another package")
			},
		},
		{
			name:   "MethodSet",
			allocs: methodSetAllocs,
			call:   func() { methods = gorules.MethodSet(notFound, ef.view) },
			check:  func(tb assert.TB) { assert.Length(tb, methods, 4, "MethodSet returns the four methods of NotFound") },
		},
		{
			name: "NullaryString", call: func() { message = gorules.NullaryString(notFoundSet, errorName, ef.view) },
			check: func(tb assert.TB) { assert.NotNil(tb, message, "NullaryString finds the method Error") },
		},
		{
			name: "SentinelName", allocs: sentinelAllocs,
			call:  func() { text = errs.SentinelName(sentinelBase) },
			check: func(tb assert.TB) { assert.Equal(tb, text, sentinel, "SentinelName spells the sentinel") },
		},
		{
			name: "SentinelBase", call: func() { text, reported = errs.SentinelBase(sentinel) },
			check: func(tb assert.TB) {
				assert.True(tb, reported, "SentinelBase recognises the sentinel")
				assert.Equal(tb, text, notFoundName, "SentinelBase returns the base")
			},
		},
		{
			name: "ErrorContract", allocs: errorContractAllocs,
			call: func() { contract, reported = errorTypes.ErrorContract(notFound, ef.view) },
			check: func(tb assert.TB) {
				assert.True(tb, reported, "ErrorContract takes NotFound as an error")
				assert.NotNil(tb, contract.Cause, "ErrorContract finds the cause")
			},
		},
		{
			name:  "Tag",
			call:  func() { text, _ = tags.Tag(tagged, jsonKey) },
			check: func(tb assert.TB) { assert.Equal(tb, text, jsonTag, "Tag returns the json value") },
		},
		{
			name: "Derive", allocs: witnessAllocs,
			call:  func() { got, _ = generics.Derive(box[0], f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, got.Spelling, witness, "Derive returns int") },
		},
		{
			name:  "Reified",
			call:  func() { reported = generics.Reified() },
			check: func(tb assert.TB) { assert.False(tb, reported, "Reified reports erasure") },
		},
		{
			name: "Settable", allocs: settableAllocs,
			call:  func() { set = promotion.Settable(derived, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, set.Members, 2, "Settable returns Name and Kind") },
		},
		{
			name: "Comparable", caseName: "a struct that does not compare", allocs: comparableAllocs,
			call: func() { reported, problems = equality.Comparable(row, f.view) },
			check: func(tb assert.TB) {
				assert.False(tb, reported, "Comparable reports that Row does not compare")
				assert.Length(tb, problems, 2, "Comparable finds the slice and the map")
			},
		},
		{
			name: "Comparable", caseName: "a struct that compares",
			call: func() { reported, problems = equality.Comparable(base, f.view) },
			check: func(tb assert.TB) {
				assert.True(tb, reported, "Comparable reports Base")
				assert.Empty(tb, problems, "Comparable finds no problem in Base")
			},
		},
	}
}

// capability returns the Go rules as the optional interface C, and
// stops the test where they do not implement it.
func capability[C any](tb assert.TB) C {
	tb.Helper()

	c, is := gorules.New().(C)
	assert.True(tb, is, "the Go rules implement the capability")
	return c
}

// paramsOf returns the type parameters of a generic struct the
// fixture declares.
func paramsOf(tb assert.TB, f *fixture, host string) []*node.TypeParam {
	tb.Helper()

	s, is := f.decl(tb, id(fxPath, host, symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, host+" is a struct")
	return s.TypeParams
}

// settable returns the set Settable returns for a struct the fixture
// declares.
func settable(tb assert.TB, host string) rules.MemberSet {
	tb.Helper()

	f := loaded(tb)
	s, is := f.decl(tb, id(fxPath, host, symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, host+" is a struct")
	return capability[rules.PromotionRules](tb).Settable(s, f.view)
}

// settableNames returns the names of the fields Settable returns for
// a struct the fixture declares, in the order it returns them.
func settableNames(tb assert.TB, host string) []string {
	tb.Helper()

	members := settable(tb, host).Members
	names := make([]string, 0, len(members))
	for _, m := range members {
		field, isField := m.Symbol.(*node.Field)
		assert.True(tb, isField, "a settable member is a field")
		names = append(names, field.Name)
	}
	return names
}

// errorsLoaded returns the error fixture with a tracked view over it.
func errorsLoaded(tb assert.TB) *fixture {
	tb.Helper()

	f := rulestest.Loaded(tb, gofrontend.New(nil), errorTree())
	return viewOver(tb, f, id(fxPath, notFoundName, symbol.KindStruct), errorsFile)
}

// errorContract returns the contract that the Go rules state for a type
// of the error fixture.
func errorContract(tb assert.TB, name string, kind symbol.Kind) (rules.ErrorContract, bool) {
	tb.Helper()

	f := errorsLoaded(tb)
	return capability[rules.ErrorTypeRules](tb).ErrorContract(f.decl(tb, id(fxPath, name, kind)), f.view)
}

// methodNames returns the names of methods, in order.
func methodNames(methods []*node.Method) []string {
	names := make([]string, 0, len(methods))
	for _, m := range methods {
		names = append(names, m.Name)
	}
	return names
}

// hiddenLoaded returns the fixture of the structs whose lowering leaves out
// a field, with a tracked view over it.
func hiddenLoaded(tb assert.TB) *fixture {
	tb.Helper()

	f := rulestest.Loaded(tb, gofrontend.New(nil), hiddenTree())
	return viewOver(tb, f, id(fxPath, paddedName, symbol.KindStruct), hiddenFile)
}

// hiddenTree returns the fixture of the structs whose lowering leaves out a
// field: a struct with a blank field, and stand-ins of a struct of the
// standard library that Go compares and of one that Go does not compare.
// Each stand-in has a blank field, so its lowering leaves out a field as
// the signature depth does for the real one.
func hiddenTree() fstest.MapFS {
	return fstest.MapFS{
		hiddenFile: {Data: []byte("package fx\n\n// Padded forbids == through a blank field.\n" +
			"type Padded struct {\n\t_ [0]func()\n\tN int\n}\n")},
		"time/time.go": {Data: []byte("package time\n\n// Time stands in for the time of the standard library.\n" +
			"type Time struct {\n\t_    [0]func()\n\twall uint64\n}\n")},
		"math/big/int.go": {Data: []byte("package big\n\n// Int stands in for the integer of the standard library.\n" +
			"type Int struct {\n\t_   int\n\tneg bool\n}\n")},
	}
}

// errorTree returns the error fixture. It declares an error with a cause
// and three hooks, an error behind a pointer, a defined type, an error
// that unwraps to many, an error whose message is an alias of string, a
// type whose method Error returns an int, an interface, an error with As
// over the empty interface, and a struct that embeds an error.
func errorTree() fstest.MapFS {
	return fstest.MapFS{
		errorsFile: {Data: []byte(`package fx

// NotFound is an error with a cause and the three hooks.
type NotFound struct {
	Key string
	err error
}

func (e NotFound) Error() string { return "not found" }

func (e NotFound) Unwrap() error { return e.err }

func (e NotFound) Is(target error) bool { return target == nil }

func (e NotFound) As(target any) bool { return target == nil }

// Timeout is an error behind a pointer.
type Timeout struct{ Op string }

func (t *Timeout) Error() string { return t.Op }

// Code is a defined type whose values are errors.
type Code int

func (c Code) Error() string { return "code" }

// Joined unwraps to many errors.
type Joined struct{ errs []error }

func (j Joined) Error() string { return "joined" }

func (j Joined) Unwrap() []error { return j.errs }

// Text is an alias of string.
type Text = string

// Message returns its message as a Text.
type Message struct{}

func (Message) Error() Text { return "message" }

// Count has a method Error that returns an int.
type Count struct{}

func (Count) Error() int { return 0 }

// Fault is an interface whose values are errors.
type Fault interface {
	Error() string
	Temporary() bool
}

// Legacy has the hook As over the empty interface.
type Legacy struct{}

func (Legacy) Error() string { return "legacy" }

func (Legacy) As(target interface{}) bool { return target == nil }

// Wrapper embeds NotFound.
type Wrapper struct{ NotFound }
`)},
	}
}

// boxOf returns a reference to the fixture's Box instantiated with
// arg, and to Box without arguments for a nil arg.
func boxOf(arg *node.TypeRef) *node.TypeRef {
	box := ref(fxPath, "Box", symbol.KindStruct)
	if arg != nil {
		box.Args = []*node.TypeRef{arg}
	}
	return box
}
