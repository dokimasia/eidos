// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"cmp"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/node/nodetest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	corestore "go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/declarative"
)

// The graph of the emit cases, in the scripted language of the kernel's
// test kits: the package svc with the interface Ledger and the function
// Transfer under the plugin's directive, and the struct Entry, which embeds
// the struct Base.
const (
	svcPath      = "svc"
	ledgerFile   = "svc/ledger.fake"
	ledgerName   = "Ledger"
	postName     = "Post"
	postDoc      = "Post adds cents to account, and returns the identifier of the entry."
	transferName = "Transfer"
	entryName    = "Entry"
	baseName     = "Base"
	idName       = "ID"
	createdName  = "Created"
	stringType   = "string"
	intType      = "int"
)

// The declarations that a case adds to the package svc: the interface
// Account with the property Owner, the generic interface Store, the
// interface Sorter with the generic method Sort, the function Reset
// without parameters, and the struct Orphan, which embeds a type that the
// graph does not contain.
const (
	accountName = "Account"
	ownerName   = "Owner"
	closeName   = "Close"
	storeName   = "Store"
	putName     = "Put"
	sorterName  = "Sorter"
	sortName    = "Sort"
	resetName   = "Reset"
	orphanName  = "Orphan"
	brokenName  = "Broken"
	missingName = "Missing"
	letterParam = "s"
	boundType   = "comparable"
)

// The values of the plugin's directive on Ledger and on Transfer.
const (
	auditDirective directive.Name     = pluginName + ":audit"
	levelKey       directive.ParamKey = "level"
	sinkKey        directive.ParamKey = "sink"
	fieldsKey      directive.ParamKey = "fields"
	strictKey      directive.ParamKey = "strict"
	infoLevel                         = "info"
	warnLevel                         = "warn"
	recordSink                        = "record"
)

// The names of the declarations that the plugin emits for the graph.
const (
	ledgerAudit    = "LedgerAudit"
	ledgerContract = "LedgerContract"
	transferAudit  = "TransferAudit"
	entryShape     = "EntryShape"
	nextField      = "next"
)

// The identities of the declarations that the cases gate with the
// plugin's directive, and of the struct Entry.
var (
	ledgerID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: svcPath, Name: ledgerName, Kind: symbol.KindInterface,
	}
	transferID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: svcPath, Name: transferName, Kind: symbol.KindFunction,
	}
	entryID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: svcPath, Name: entryName, Kind: symbol.KindStruct,
	}
	accountID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: svcPath, Name: accountName, Kind: symbol.KindInterface,
	}
	storeID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: svcPath, Name: storeName, Kind: symbol.KindInterface,
	}
	storeParamID = symbol.Identity{
		Lang:    frontendtest.ScriptedLang,
		Package: svcPath,
		Owner:   storeName,
		Name:    letterParam,
		Kind:    symbol.KindTypeParam,
	}
	sorterID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: svcPath, Name: sorterName, Kind: symbol.KindInterface,
	}
	brokenID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: svcPath, Name: brokenName, Kind: symbol.KindInterface,
	}
)

// The data contract of the template references: the template of the cases
// reads each field of the data that a template of a plugin reads.
const dataTemplate = "{{.Data.Subject}}|{{.Data.Directive.level}}|{{.Data.Directive.sink}}|{{.Data.Options.redact}}"

// The scaled corpus of the benchmark: packages of source files of
// declarations, 200,000 in all, and the allocation budget of one generate
// call over it.
const (
	benchPackages  = 1_000
	benchFiles     = 10
	benchDecls     = 20
	generateBudget = 540_000
)

// benchManifest is the manifest of the benchmark: the two rules of the
// conformance fixture, which implement each interface and mirror each
// function under the directive, without options.
const benchManifest = `format: 1
name: acme-audit
version: 0.3.0
directives:
    - name: audit
      doc: Generates code that records each call of the declaration.
      params:
          - key: level
            type: string
            choices: [debug, info, warn]
            required: true
            doc: The level of each record.
          - key: sink
            type: reference
            resolve: callable-in-scope
            required: true
            doc: The function that receives each record.
outputs:
    - per: source
      word: audit
rules:
    - on: interface
      directive: audit
      emit: implementation
      template: method.tmpl
    - on: function
      directive: audit
      emit: mirror
      template: function.tmpl
`

// recordManifest is the manifest of a plugin without options whose one
// family claims its files. Its bare rules emit the mirror of each interface
// and of each struct as a record of the family.
const recordManifest = `format: 1
name: acme-audit
version: 0.3.0
outputs:
    - tag: test
      per: source
      word: audit
      template: test.tmpl
rules:
    - on: interface
      output: test
      emit: mirror
    - on: struct
      output: test
      emit: mirror
`

// The generate phase emits one declaration for each rule and subject,
// routes it to the rule's family, and claims each body for the rule's
// template with the data that a template reads.
func TestEmit(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("emits the implementation of an interface under its directive", func(t *testing.T) {
			t.Parallel()

			impl := only[*emit.Struct](t, generated(t, auditPlugin(t), ledgerFixture(t)), "")
			assert.Equal(t, impl.Name, ledgerAudit, "the name joins the family's word to the subject's name")
			expect.Equal(t, impl.Origin, ledgerID, "the struct derives from the interface")
			expect.Equal(t, impl.Implements, []*emit.TypeRef{{Spelling: ledgerName, Target: ledgerID}},
				"the struct implements the interface")
			fields := impl.Fields.Items()
			assert.Length(t, fields, 1, "the struct has one field")
			expect.Equal(t, fields[0].Name, nextField, "the field has the name next")
			expect.Equal(t, fields[0].Visibility, symbol.VisibilityPackage, "the field is private to the package")
			expect.Equal(t, fields[0].Type, &emit.TypeRef{Spelling: ledgerName, Target: ledgerID},
				"the field next has the interface's type")
			expect.Empty(t, fields[0].Tags.Items(), "the field has no tag")
		})

		t.Run("emits a method of an implementation on a receiver with the letter of the struct", func(t *testing.T) {
			t.Parallel()

			impl := only[*emit.Struct](t, generated(t, auditPlugin(t), ledgerFixture(t)), "")
			methods := impl.Methods.Items()
			assert.Length(t, methods, 1, "the struct has a method for each method of the interface")
			post := methods[0]
			expect.Equal(t, post.Name, postName, "the method keeps its name")
			expect.Equal(t, post.Doc, []string{postDoc}, "the method keeps its documentation")
			expect.Equal(t, post.Receiver, &emit.Param{Name: "l", Type: &emit.TypeRef{Spelling: ledgerAudit}},
				"the receiver has the first letter of the struct's name and the struct's type")
			expect.Equal(t, post.Params.Items(), []*emit.Param{
				{Name: "account", Type: &emit.TypeRef{Spelling: stringType}},
				{Name: "cents", Type: &emit.TypeRef{Spelling: intType}},
			}, "the method keeps the parameters of the interface's method")
			assert.NotNil(t, post.Body.Ref, "the method claims its body")
			expect.Equal(t, post.Body.Ref.Name, methodTemplate, "the rule's template renders the body")
			expect.Equal(t, post.Body.Ref.Owner, pluginName, "the plugin's tree contains the template")
		})

		t.Run("names the receiver apart from a parameter with its letter", func(t *testing.T) {
			t.Parallel()

			f := ledgerFixture(t, lookup())
			lookupID := symbol.Identity{
				Lang:    frontendtest.ScriptedLang,
				Package: svcPath,
				Name:    "Lookup",
				Kind:    symbol.KindInterface,
			}
			f.Validated(t, lookupID, audit(warnLevel, position.Pos{File: ledgerFile, Line: 20, Col: 1}))
			var receiver string
			for _, s := range declarations[*emit.Struct](generated(t, auditPlugin(t), f), "") {
				if s.Name == "LookupAudit" {
					receiver = s.Methods.Items()[0].Receiver.Name
				}
			}
			assert.Equal(t, receiver, "l0", "the receiver takes the smallest number that no parameter has")
		})

		t.Run("emits the mirror of a function under its directive", func(t *testing.T) {
			t.Parallel()

			var mirror *emit.Function
			for _, fn := range declarations[*emit.Function](generated(t, auditPlugin(t), ledgerFixture(t)), "") {
				mirror = fn
			}
			assert.NotNil(t, mirror, "the rule emits a function")
			expect.Equal(t, mirror.Name, transferAudit, "the name joins the family's word to the subject's name")
			expect.Equal(t, mirror.Params.Items(), []*emit.Param{
				{Name: "from", Type: &emit.TypeRef{Spelling: stringType}},
				{Name: "cents", Type: &emit.TypeRef{Spelling: intType}},
			}, "the function keeps the parameters of the subject")
			assert.NotNil(t, mirror.Body.Ref, "the function claims its body")
			expect.Equal(t, mirror.Body.Ref.Name, functionTemplate, "the rule's template renders the body")
		})

		t.Run("passes the subject, the params and the options to a template", func(t *testing.T) {
			t.Parallel()

			impl := only[*emit.Struct](t, generated(t, auditPlugin(t), ledgerFixture(t)), "")
			post := impl.Methods.Items()[0]
			assert.Equal(t, executed(t, dataTemplate, post, post.Body.Ref.Data), "Ledger|info|record|true",
				"the data contains the subject's name, each param and each option")
		})

		t.Run("passes nil for an optional param that the instance does not write", func(t *testing.T) {
			t.Parallel()

			impl := only[*emit.Struct](t, generated(t, auditPlugin(t), ledgerFixture(t)), "")
			post := impl.Methods.Items()[0]
			assert.Equal(
				t,
				executed(t, "{{if .Data.Directive.fields}}set{{else}}unset{{end}}", post, post.Body.Ref.Data),
				"unset",
				"the data contains the key of the param with no value",
			)
		})

		t.Run("passes a list param as a list of its elements", func(t *testing.T) {
			t.Parallel()

			f := ledgerFixture(t)
			listed := audit(warnLevel, position.Pos{File: ledgerFile, Line: 8, Col: 1})
			listed.Params[fieldsKey] = directive.Value{Kind: directive.TypeList, List: []directive.Value{
				{Kind: directive.TypeInt, Int: 1}, {Kind: directive.TypeInt, Int: 2},
			}}
			f.Directives[transferID] = []directive.Directive{listed}
			var data any
			for _, fn := range declarations[*emit.Function](generated(t, auditPlugin(t), f), "") {
				data = fn.Body.Ref.Data
			}
			assert.Equal(t, executed(t, `{{range .Data.Directive.fields}}{{printf "%T=%v " . .}}{{end}}`, nil, data),
				"int64=1 int64=2 ", "each element of an int list is an int64")
		})

		t.Run("passes a bool param as a bool", func(t *testing.T) {
			t.Parallel()

			f := ledgerFixture(t)
			strict := audit(warnLevel, position.Pos{File: ledgerFile, Line: 8, Col: 1})
			strict.Params[strictKey] = directive.Value{Kind: directive.TypeBool, Bool: true}
			f.Directives[transferID] = []directive.Directive{strict}
			var data any
			for _, fn := range declarations[*emit.Function](generated(t, auditPlugin(t), f), "") {
				data = fn.Body.Ref.Data
			}
			assert.Equal(t, executed(t, `{{printf "%T=%v" .Data.Directive.strict .Data.Directive.strict}}`, nil, data),
				"bool=true", "the data contains the bool of the instance")
		})

		t.Run("passes no params to the template of a bare rule", func(t *testing.T) {
			t.Parallel()

			var data any
			for _, fn := range declarations[*emit.Function](generated(t, auditPlugin(t), ledgerFixture(t)), contractTag) {
				data = fn.Body.Ref.Data
			}
			assert.Equal(t, executed(t, "{{len .Data.Directive}}|{{.Data.Subject}}", nil, data), "0|Transfer",
				"a bare rule has no gating directive")
		})

		t.Run("emits a record with the data of its match into a family that claims its files", func(t *testing.T) {
			t.Parallel()

			var got []plugin.Unit
			for u := range generated(t, auditPlugin(t), ledgerFixture(t)).Emit.Units() {
				if u.Tag == testTag {
					got = append(got, u)
				}
			}
			assert.Length(t, got, 1, "the family has one unit")
			assert.Equal(t, got[0].Template, testTemplate, "the unit has the family's file template")
			assert.Length(t, got[0].Decls, 1, "the unit has the record of Transfer")
			assert.Equal(
				t,
				executed(t, "{{.Data.Subject}}|{{.Data.Directive.level}}", got[0].Decls[0], got[0].Data[transferID]),
				"Transfer|warn",
				"the record has the data of its match",
			)
		})

		t.Run("claims no body of a record whose rule has no template", func(t *testing.T) {
			t.Parallel()

			record := only[*emit.Function](t, generated(t, auditPlugin(t), ledgerFixture(t)), testTag)
			assert.Nil(t, record.Body.Ref, "the file template writes the record whole")
		})

		t.Run("emits the mirror of an interface with each effective method", func(t *testing.T) {
			t.Parallel()

			var mirror *emit.Interface
			for _, i := range declarations[*emit.Interface](generated(t, auditPlugin(t), ledgerFixture(t)), contractTag) {
				mirror = i
			}
			assert.NotNil(t, mirror, "the rule emits an interface")
			expect.Equal(t, mirror.Name, ledgerContract, "the name joins the family's word to the subject's name")
			methods := mirror.Methods.Items()
			assert.Length(t, methods, 1, "the mirror has a method for each method of the interface")
			expect.Nil(t, methods[0].Receiver, "the method of an interface has no receiver")
			expect.Nil(t, methods[0].Body.Ref, "the method of an interface has no body")
		})

		t.Run("emits the mirror of a struct with each effective field", func(t *testing.T) {
			t.Parallel()

			var mirror *emit.Struct
			for _, s := range declarations[*emit.Struct](generated(t, auditPlugin(t), ledgerFixture(t)), shapeTag) {
				if s.Name == entryShape {
					mirror = s
				}
			}
			assert.NotNil(t, mirror, "the name joins the family's word to the subject's name")
			names := make([]string, 0, mirror.Fields.Len())
			for _, f := range mirror.Fields.Items() {
				names = append(names, f.Name)
			}
			assert.Equal(t, names, []string{idName, createdName},
				"the mirror has the struct's own field and the field that its embed promotes")
		})

		t.Run("emits the mirror of an interface with each effective property", func(t *testing.T) {
			t.Parallel()

			var mirror *emit.Interface
			for _, i := range declarations[*emit.Interface](generated(t, auditPlugin(t), ledgerFixture(t, account())), contractTag) {
				if i.Name == accountName+"Contract" {
					mirror = i
				}
			}
			assert.NotNil(t, mirror, "the rule mirrors the interface")
			fields := mirror.Fields.Items()
			assert.Length(t, fields, 1, "the mirror has the property of the interface")
			expect.Equal(t, fields[0].Origin, symbol.Identity{
				Lang:    frontendtest.ScriptedLang,
				Package: svcPath,
				Owner:   accountName,
				Name:    ownerName,
				Kind:    symbol.KindField,
			}, "the property derives from the property of the interface")
			expect.Equal(t, fields[0].Name, ownerName, "the property keeps its name")
			expect.Equal(t, fields[0].Visibility, symbol.VisibilityPublic, "the property keeps its visibility")
			expect.Equal(t, fields[0].Type, &emit.TypeRef{Spelling: stringType}, "the property keeps its type")
			expect.Empty(t, fields[0].Tags.Items(), "the property has no tag")
			expect.Length(t, mirror.Methods.Items(), 1, "the mirror has the method of the interface")
		})

		t.Run("emits the mirror of a function without parameters", func(t *testing.T) {
			t.Parallel()

			var mirror *emit.Function
			for _, fn := range declarations[*emit.Function](generated(t, auditPlugin(t), ledgerFixture(t, reset())), contractTag) {
				if fn.Name == resetName+"Contract" {
					mirror = fn
				}
			}
			assert.NotNil(t, mirror, "the bare rule mirrors every function")
			assert.Empty(t, mirror.Params.Items(), "the mirror has no parameter")
		})

		t.Run("emits the implementation of a generic interface with its type parameters", func(t *testing.T) {
			t.Parallel()

			impl := structNamed(generated(t, auditPlugin(t), gated(t, storeID, store())), "", storeName+"Audit")
			assert.NotNil(t, impl, "the rule implements the interface")
			expect.Equal(t, impl.TypeParams, []*emit.TypeParam{{
				Name: letterParam, Bounds: []*emit.TypeRef{{Spelling: boundType}},
			}}, "the struct has the type parameter of the interface with its bound")
			expect.Equal(t, impl.Implements, []*emit.TypeRef{
				{
					Spelling: storeName,
					Target:   storeID,
					Args:     []*emit.TypeRef{{Spelling: letterParam, Target: storeParamID}},
				},
			}, "the struct implements the interface over the type parameter of the interface")
		})

		t.Run("names the receiver apart from a type parameter of the struct with its letter", func(t *testing.T) {
			t.Parallel()

			impl := structNamed(generated(t, auditPlugin(t), gated(t, storeID, store())), "", storeName+"Audit")
			assert.NotNil(t, impl, "the rule implements the interface")
			assert.Equal(t, impl.Methods.Items()[0].Receiver, &emit.Param{
				Name: letterParam + "0",
				Type: &emit.TypeRef{Spelling: storeName + "Audit", Args: []*emit.TypeRef{{Spelling: letterParam}}},
			}, "the receiver takes the smallest number that no type parameter has")
		})

		t.Run("names the receiver apart from a type parameter of the method with its letter", func(t *testing.T) {
			t.Parallel()

			impl := structNamed(generated(t, auditPlugin(t), gated(t, sorterID, sorter())), "", sorterName+"Audit")
			assert.NotNil(t, impl, "the rule implements the interface")
			assert.Equal(t, impl.Methods.Items()[0].Receiver.Name, letterParam+"0",
				"the receiver takes the smallest number that no type parameter of the method has")
		})

		t.Run("emits a record of the mirror of an interface into a family that claims its files", func(t *testing.T) {
			t.Parallel()

			units := recordUnits(t)
			assert.Length(t, units, 1, "the family has one unit for the one source file")
			assert.Equal(t, executed(t, "{{.Data.Subject}}", nil, units[0].Data[ledgerID]), ledgerName,
				"the record of the interface has the data of its match")
		})

		t.Run("emits a record of the mirror of a struct into a family that claims its files", func(t *testing.T) {
			t.Parallel()

			units := recordUnits(t)
			assert.Length(t, units, 1, "the family has one unit for the one source file")
			assert.Equal(t, executed(t, "{{.Data.Subject}}", nil, units[0].Data[entryID]), entryName,
				"the record of the struct has the data of its match")
		})

		tests := []struct {
			name string
			tag  string
			want plugin.Cardinality
			key  string
		}{
			{
				name: "routes a declaration of a family per source to the subject's file",
				want: plugin.PerSource,
				key:  ledgerFile,
			},
			{
				name: "routes a declaration of a family per package to the subject's package",
				tag:  contractTag,
				want: plugin.PerPackage,
				key:  svcPath,
			},
			{name: "routes a declaration of a family per plan to the plan", tag: shapeTag, want: plugin.PerPlan},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got []plugin.Unit
				for u := range generated(t, auditPlugin(t), ledgerFixture(t)).Emit.Units() {
					if u.Tag == tt.tag {
						got = append(got, u)
					}
				}
				assert.NotEmpty(t, got, "the family has a unit")
				for _, u := range got {
					expect.Equal(t, u.Per, tt.want, "the unit has the family's cardinality")
					expect.Equal(t, u.Key, tt.key, "the unit has the key of the cardinality")
				}
			})
		}

		t.Run("reports IncompleteMembers for an interface whose embed does not resolve", func(t *testing.T) {
			t.Parallel()

			res := generated(t, auditPlugin(t), ledgerFixture(t, broken()))
			found := false
			for d := range res.Sink.All() {
				if d.Code == declarative.IncompleteMembers {
					found = true
					expect.Contains(t, d.Msg, "Missing (unresolved)", "the message of the finding has the contributor")
				}
			}
			assert.True(t, found, "the run reports the incomplete members")
		})

		t.Run("emits nothing for an interface whose embed does not resolve", func(t *testing.T) {
			t.Parallel()

			for _, i := range declarations[*emit.Interface](generated(t, auditPlugin(t), ledgerFixture(t, broken())), contractTag) {
				assert.NotEqual(t, i.Name, "BrokenContract", "the plugin does not mirror an incomplete interface")
			}
		})

		t.Run("emits no implementation of an interface whose embed does not resolve", func(t *testing.T) {
			t.Parallel()

			res := generated(t, auditPlugin(t), gated(t, brokenID, broken()))
			assert.Nil(
				t,
				structNamed(res, "", brokenName+"Audit"),
				"the plugin does not implement an incomplete interface",
			)
		})

		t.Run("emits nothing for a struct whose embed does not resolve", func(t *testing.T) {
			t.Parallel()

			res := generated(t, auditPlugin(t), ledgerFixture(t, orphan()))
			assert.Nil(
				t,
				structNamed(res, shapeTag, orphanName+"Shape"),
				"the plugin does not mirror an incomplete struct",
			)
		})

		t.Run("reports IncompleteMembers for a struct whose embed does not resolve", func(t *testing.T) {
			t.Parallel()

			res := generated(t, auditPlugin(t), ledgerFixture(t, orphan()))
			assert.Contains(t, codes(res), declarative.IncompleteMembers, "the run reports the incomplete members")
		})

		t.Run("reports UndelegatedProperty for an interface with a property", func(t *testing.T) {
			t.Parallel()

			var got []diag.Diag
			for d := range generated(t, auditPlugin(t), gated(t, accountID, account())).Sink.All() {
				if d.Code == declarative.UndelegatedProperty {
					got = append(got, d)
				}
			}
			assert.Length(t, got, 1, "the run reports the interface once")
			assert.Equal(
				t,
				got[0].Msg,
				"an implementation of Account cannot delegate its properties, and the plugin emits nothing for it: Owner",
				"the message of the finding has the property",
			)
		})

		t.Run("emits no implementation of an interface with a property", func(t *testing.T) {
			t.Parallel()

			res := generated(t, auditPlugin(t), gated(t, accountID, account()))
			assert.Nil(t, structNamed(res, "", accountName+"Audit"), "a property has no call to delegate")
		})

		t.Run("reports RefusedType for a type that the target cannot spell", func(t *testing.T) {
			t.Parallel()

			f := ledgerFixture(t)
			f.Target = "other"
			found := false
			for d := range generated(t, auditPlugin(t), f).Sink.All() {
				found = found || d.Code == eidos.RefusedType
			}
			assert.True(t, found, "the target without a spoke refuses every type")
		})

		t.Run("emits nothing for a declaration with a type that the target cannot spell", func(t *testing.T) {
			t.Parallel()

			f := ledgerFixture(t)
			f.Target = "other"
			var units []plugin.Unit
			for u := range generated(t, auditPlugin(t), f).Emit.Units() {
				units = append(units, u)
			}
			assert.Empty(t, units, "every declaration of the graph has a type")
		})
	})
}

// BenchmarkEmit measures the generate phase of a declarative plugin with the
// two rules of the conformance fixture over the scaled corpus: 1,000
// packages of 10 source files of 20 declarations, 200,000 in all. The
// directive gates one interface and one function of each file, so a call
// implements 10,000 interfaces and mirrors 10,000 functions.
//
// One run at -benchtime=1x measured 510,608 allocations, about 25 for each
// match: the declaration, its members, the translated types, the template
// reference of each body, and the payload with the map of the directive's
// params. The budget is 540,000, about 5% above. Each iteration takes a new
// emit store, sink and reader outside the count through Excluding, which no
// MaxAllocs count can leave out, so only -bench checks the budget.
func BenchmarkEmit(b *testing.B) {
	b.Run("Generate", func(b *testing.B) {
		b.Run("the scaled corpus", func(b *testing.B) {
			g, err := declarative.Load(files.Workspace(b, files.Tree{
				manifestFile: files.Text(benchManifest),
				fakeFunction: files.Text(functionBody),
				fakeMethod:   files.Text(methodBody),
			}))
			assert.NoError(b, err, "the plugin directory loads")
			f := scaledCorpus(b)
			f.Graph.Freeze()
			ix, err := plugin.NewIndex(f.Graph, f.Facts, plugin.ValidatedMap(f.Directives), f.Scope)
			assert.NoError(b, err, "the index of the corpus builds")

			c := bench.Start(b).MaxAllocs(generateBudget)
			defer c.End()
			var genErr error
			units := 0
			for c.Loop() {
				var ctx *plugin.GeneratorContext
				c.Excluding(func() {
					r, rerr := ix.Reader(corestore.NewReadSet())
					assert.NoError(b, rerr, "the reader of the call mints")
					ctx = &plugin.GeneratorContext{
						Index: ix, Reader: r, Facts: f.Facts, Emit: plugin.NewEmit(), Sink: diag.NewSink(),
						Rules: f.Rules, Kernel: f.Kernel, Plugin: g.Name(), Turn: 1, Target: f.Target,
					}
				})
				genErr = cmp.Or(genErr, g.Generate(ctx))
				units = 0
				for range ctx.Emit.Units() {
					units++
				}
			}
			assert.NoError(b, genErr, "every call generates")
			assert.Equal(b, units, benchPackages*benchFiles, "each call emits one unit for each source file")
		})
	})
}

// auditPlugin returns the generator that Load returns for the plugin
// directory of auditTree.
func auditPlugin(tb testing.TB) plugin.Generator {
	tb.Helper()

	g, err := declarative.Load(files.Workspace(tb, auditTree()))
	assert.NoError(tb, err, "the plugin directory loads")
	return g
}

// ledgerFixture returns a fixture of the scripted language with the package
// svc and the gating instances of the plugin's directive on Ledger and on
// Transfer. extra adds declarations to the package. The plan's target is
// the scripted language, so each type translates as it is written.
func ledgerFixture(tb assert.TB, extra ...symbol.Symbol) *plugintest.Fixture {
	tb.Helper()

	f := plugintest.New(tb)
	f.Rules = rules.NewRegistry()
	assert.NoError(tb, f.Rules.Register(rulestest.Scripted()), "the scripted rules register")
	f.Target = plugin.Target(frontendtest.ScriptedLang)
	f.Load(tb, ledgerPackage(tb, extra...)...)
	f.Validated(tb, ledgerID, audit(infoLevel, position.Pos{File: ledgerFile, Line: 2, Col: 1}))
	f.Validated(tb, transferID, audit(warnLevel, position.Pos{File: ledgerFile, Line: 8, Col: 1}))
	return f
}

// ledgerPackage builds the package svc of the scripted language through
// nodetest: the interface Ledger, the function Transfer, the struct Base
// and the struct Entry, which embeds Base, and the declarations of extra.
func ledgerPackage(tb assert.TB, extra ...symbol.Symbol) []*node.Package {
	tb.Helper()

	b := nodetest.New(nodetest.Neutral{Language: frontendtest.ScriptedLang})
	post := &node.Method{
		Pos:        position.Pos{File: ledgerFile, Line: 5, Col: 2},
		Doc:        []string{postDoc},
		Name:       postName,
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Abstract:   true,
		Params: []*node.Param{
			{Name: "account", Type: nodetest.Named(stringType)},
			{Name: "cents", Type: nodetest.Named(intType)},
		},
		Returns: []*node.Return{{Type: nodetest.Named(stringType)}},
	}
	ledger := &node.Interface{
		Pos:        position.Pos{File: ledgerFile, Line: 3, Col: 1},
		Name:       ledgerName,
		Visibility: symbol.VisibilityPublic,
		Methods:    []*node.Method{post},
	}
	transfer := &node.Function{
		Pos:        position.Pos{File: ledgerFile, Line: 9, Col: 1},
		Name:       transferName,
		Visibility: symbol.VisibilityPublic,
		Params: []*node.Param{
			{Name: "from", Type: nodetest.Named(stringType)},
			{Name: "cents", Type: nodetest.Named(intType)},
		},
	}
	base := &node.Struct{
		Pos:        position.Pos{File: ledgerFile, Line: 12, Col: 1},
		Name:       baseName,
		Visibility: symbol.VisibilityPackage,
		Fields: []*node.Field{{
			Name: createdName, Visibility: symbol.VisibilityPublic, Type: nodetest.Named(intType),
		}},
	}
	entry := &node.Struct{
		Pos:        position.Pos{File: ledgerFile, Line: 15, Col: 1},
		Name:       entryName,
		Visibility: symbol.VisibilityPublic,
		Fields: []*node.Field{{
			Name: idName, Visibility: symbol.VisibilityPublic, Type: nodetest.Named(stringType),
		}},
		Embeds: []*node.Embed{{Ref: b.Ref(base)}},
	}
	b.File(svcPath, ledgerFile, append([]symbol.Symbol{ledger, transfer, base, entry}, extra...)...)
	return b.Build(tb)
}

// lookup returns the interface Lookup, whose method Get has the parameter
// l, the first letter of the implementation LookupAudit.
func lookup() *node.Interface {
	return &node.Interface{
		Pos:        position.Pos{File: ledgerFile, Line: 21, Col: 1},
		Name:       "Lookup",
		Visibility: symbol.VisibilityPublic,
		Methods: []*node.Method{{
			Pos:        position.Pos{File: ledgerFile, Line: 22, Col: 2},
			Name:       "Get",
			Visibility: symbol.VisibilityPublic,
			Params:     []*node.Param{{Name: "l", Type: nodetest.Named(stringType)}},
		}},
	}
}

// broken returns the interface Broken, which embeds the type Missing that
// the graph does not contain.
func broken() *node.Interface {
	return &node.Interface{
		Pos:        position.Pos{File: ledgerFile, Line: 25, Col: 1},
		Name:       brokenName,
		Visibility: symbol.VisibilityPublic,
		Embeds:     []*node.Embed{{Ref: nodetest.Named(missingName)}},
	}
}

// orphan returns the struct Orphan, which embeds the type Missing that the
// graph does not contain.
func orphan() *node.Struct {
	return &node.Struct{
		Pos:        position.Pos{File: ledgerFile, Line: 27, Col: 1},
		Name:       orphanName,
		Visibility: symbol.VisibilityPublic,
		Embeds:     []*node.Embed{{Ref: nodetest.Named(missingName)}},
	}
}

// account returns the interface Account with the property Owner and the
// method Close.
func account() *node.Interface {
	return &node.Interface{
		Pos:        position.Pos{File: ledgerFile, Line: 29, Col: 1},
		Name:       accountName,
		Visibility: symbol.VisibilityPublic,
		Fields: []*node.Field{{
			Name: ownerName, Visibility: symbol.VisibilityPublic, Type: nodetest.Named(stringType),
		}},
		Methods: []*node.Method{{Name: closeName, Visibility: symbol.VisibilityPublic}},
	}
}

// store returns the generic interface Store with the type parameter s,
// whose bound is comparable, and the method Put, which takes a value of s.
// The type parameter has the first letter of the implementation StoreAudit.
func store() *node.Interface {
	return &node.Interface{
		Pos:        position.Pos{File: ledgerFile, Line: 33, Col: 1},
		Name:       storeName,
		Visibility: symbol.VisibilityPublic,
		TypeParams: []*node.TypeParam{{
			Name:   letterParam,
			Bounds: []*node.TypeRef{nodetest.Named(boundType)},
		}},
		Methods: []*node.Method{{
			Name:       putName,
			Visibility: symbol.VisibilityPublic,
			Params:     []*node.Param{{Name: "v", Type: nodetest.Named(letterParam)}},
		}},
	}
}

// sorter returns the interface Sorter with the method Sort, whose type
// parameter s has the first letter of the implementation SorterAudit.
func sorter() *node.Interface {
	return &node.Interface{
		Pos:        position.Pos{File: ledgerFile, Line: 37, Col: 1},
		Name:       sorterName,
		Visibility: symbol.VisibilityPublic,
		Methods: []*node.Method{{
			Name:       sortName,
			Visibility: symbol.VisibilityPublic,
			TypeParams: []*node.TypeParam{{Name: letterParam}},
		}},
	}
}

// reset returns the function Reset without parameters.
func reset() *node.Function {
	return &node.Function{
		Pos:        position.Pos{File: ledgerFile, Line: 40, Col: 1},
		Name:       resetName,
		Visibility: symbol.VisibilityPublic,
	}
}

// gated returns a fixture of ledgerFixture with extra in the package and a
// gating instance of the plugin's directive on the declaration id, which
// extra declares.
func gated(tb assert.TB, id symbol.Identity, extra symbol.Symbol) *plugintest.Fixture {
	tb.Helper()

	f := ledgerFixture(tb, extra)
	f.Validated(tb, id, audit(infoLevel, extra.Position()))
	return f
}

// structNamed returns the struct of the family tag that res emitted under
// the given name, and nil where the family has none.
func structNamed(res plugintest.Result, tag, name string) *emit.Struct {
	for _, s := range declarations[*emit.Struct](res, tag) {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// recordUnits loads the plugin directory of recordManifest, runs its
// generator over ledgerFixture, and returns the units that it emitted.
func recordUnits(tb testing.TB) []plugin.Unit {
	tb.Helper()

	g, err := declarative.Load(files.Workspace(tb, files.Tree{
		manifestFile: files.Text(recordManifest),
		fakeTest:     files.Text(testBody),
	}))
	assert.NoError(tb, err, "the plugin directory loads")
	var out []plugin.Unit
	for u := range generated(tb, g, ledgerFixture(tb)).Emit.Units() {
		out = append(out, u)
	}
	return out
}

// codes returns the code of each finding that res reported, in the order
// of the sink.
func codes(res plugintest.Result) []diag.Code {
	var out []diag.Code
	for d := range res.Sink.All() {
		out = append(out, d.Code)
	}
	return out
}

// audit returns a validated instance of the plugin's directive at the
// position at, with the level and the sink record.
func audit(level string, at position.Pos) directive.Directive {
	return directive.Directive{
		Name: auditDirective,
		Params: map[directive.ParamKey]directive.Value{
			levelKey: {Kind: directive.TypeString, Str: level},
			sinkKey:  {Kind: directive.TypeReference, Ref: recordSink},
		},
		Pos: at,
	}
}

// generated runs the generate phase of g over f, and fails the test where
// the phase returns an error.
func generated(tb assert.TB, g plugin.Generator, f *plugintest.Fixture) plugintest.Result {
	tb.Helper()

	res := f.Generate(tb, g)
	assert.NoError(tb, res.Err, "the generate phase runs")
	return res
}

// declarations returns each declaration of type D of the units of the
// family tag that res emitted.
func declarations[D symbol.Symbol](res plugintest.Result, tag string) []D {
	var out []D
	for u := range res.Emit.Units() {
		if u.Tag != tag {
			continue
		}
		for _, d := range u.Decls {
			if decl, is := d.(D); is {
				out = append(out, decl)
			}
		}
	}
	return out
}

// only returns the one declaration of type D of the family tag that res
// emitted, and fails the test where the family has another number of them.
func only[D symbol.Symbol](tb assert.TB, res plugintest.Result, tag string) D {
	tb.Helper()

	got := declarations[D](res, tag)
	assert.Length(tb, got, 1, "the family has one declaration of the type")
	return got[0]
}

// scaledCorpus returns a fixture of the scripted language with benchPackages
// packages of benchFiles source files of benchDecls declarations. The
// declarations of a file alternate an interface with the method Post and a
// function of two parameters, and the plugin's directive gates the first
// interface and the first function of each file.
func scaledCorpus(tb testing.TB) *plugintest.Fixture {
	tb.Helper()

	lang := frontendtest.ScriptedLang
	f := plugintest.New(tb)
	f.Rules = rules.NewRegistry()
	assert.NoError(tb, f.Rules.Register(rulestest.Scripted()), "the scripted rules register")
	f.Target = plugin.Target(lang)
	var gated []symbol.Identity
	for p := range benchPackages {
		pkg := "p" + strconv.Itoa(p)
		out := &node.Package{
			ID:   symbol.Identity{Lang: lang, Package: pkg, Kind: symbol.KindPackage},
			Path: []string{pkg},
		}
		for file := range benchFiles {
			path := pkg + "/f" + strconv.Itoa(file) + ".fake"
			src := &node.File{
				ID:   symbol.Identity{Lang: lang, Package: pkg, Name: path, Kind: symbol.KindFile},
				Path: path,
			}
			for d := range benchDecls {
				name := "D" + strconv.Itoa(file) + "x" + strconv.Itoa(d)
				at := position.Pos{File: path, Line: d + 1, Col: 1}
				decl := scaledInterface(pkg, name, at)
				if d%2 == 1 {
					decl = scaledFunction(pkg, name, at)
				}
				src.Decls = append(src.Decls, decl)
				if d < 2 {
					gated = append(gated, decl.Identity())
				}
			}
			out.Files = append(out.Files, src)
		}
		f.Load(tb, out)
	}
	for _, id := range gated {
		f.Validated(tb, id, audit(infoLevel, position.Pos{File: id.Package + "/f0.fake", Line: 1, Col: 1}))
	}
	return f
}

// scaledInterface returns the interface name of the package pkg at the
// position at, with the method Post of the parameters account and cents.
func scaledInterface(pkg, name string, at position.Pos) node.Declaration {
	lang := frontendtest.ScriptedLang
	id := symbol.Identity{Lang: lang, Package: pkg, Name: name, Kind: symbol.KindInterface}
	owner := name + "." + postName
	return &node.Interface{
		ID: id, Pos: at, Name: name, Visibility: symbol.VisibilityPublic,
		Methods: []*node.Method{{
			ID:   symbol.Identity{Lang: lang, Package: pkg, Owner: name, Name: postName, Kind: symbol.KindMethod},
			Name: postName, Visibility: symbol.VisibilityPublic, Host: id,
			Params: []*node.Param{
				{
					ID: symbol.Identity{
						Lang:    lang,
						Package: pkg,
						Owner:   owner,
						Name:    "account",
						Kind:    symbol.KindParam,
					},
					Name: "account",
					Type: &node.TypeRef{Spelling: stringType},
				},
				{
					ID: symbol.Identity{
						Lang:    lang,
						Package: pkg,
						Owner:   owner,
						Name:    "cents",
						Kind:    symbol.KindParam,
					},
					Name: "cents",
					Type: &node.TypeRef{Spelling: intType},
				},
			},
			Returns: []*node.Return{{
				ID:   symbol.Identity{Lang: lang, Package: pkg, Owner: owner, Name: "0", Kind: symbol.KindReturn},
				Type: &node.TypeRef{Spelling: stringType},
			}},
		}},
	}
}

// scaledFunction returns the function name of the package pkg at the
// position at, with the parameters from and cents.
func scaledFunction(pkg, name string, at position.Pos) node.Declaration {
	lang := frontendtest.ScriptedLang
	return &node.Function{
		ID: symbol.Identity{Lang: lang, Package: pkg, Name: name, Kind: symbol.KindFunction}, Pos: at, Name: name,
		Visibility: symbol.VisibilityPublic,
		Params: []*node.Param{
			{
				ID:   symbol.Identity{Lang: lang, Package: pkg, Owner: name, Name: "from", Kind: symbol.KindParam},
				Name: "from", Type: &node.TypeRef{Spelling: stringType},
			},
			{
				ID:   symbol.Identity{Lang: lang, Package: pkg, Owner: name, Name: "cents", Kind: symbol.KindParam},
				Name: "cents", Type: &node.TypeRef{Spelling: intType},
			},
		},
	}
}

// executed executes the template text with missingkey=error over the data
// that the render passes to a reference template, and returns the output.
func executed(tb assert.TB, text string, decl, data any) string {
	tb.Helper()

	tmpl, err := template.New("data").Option("missingkey=error").Parse(text)
	assert.NoError(tb, err, "the template parses")
	var out strings.Builder
	assert.NoError(tb, tmpl.Execute(&out, struct{ Decl, Data any }{Decl: decl, Data: data}), "the template executes")
	return out.String()
}
