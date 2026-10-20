// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/rust"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The ceilings of the values. The pair of a number and the zero of a number
// allocate nothing, and the pair of a String allocates its two hinted texts
// and the argument list of each call. The literal and the text of a String
// allocate the argument list of their call, and the text of a char allocates
// its quoted spelling.
const (
	numberPairAllocs = 0
	stringPairAllocs = 4
	zeroAllocs       = 0
	stringLitAllocs  = 1
	stringTextAllocs = 1
	charTextAllocs   = 1
)

// The items of the standard library that the values call and name.
var (
	stringFrom = symbol.Identity{Lang: rust.Lang, Owner: stringName, Name: "from", Kind: symbol.KindMethod}
	stringNew  = symbol.Identity{Lang: rust.Lang, Owner: stringName, Name: "new", Kind: symbol.KindMethod}
	someCall   = symbol.Identity{Lang: rust.Lang, Name: "Some", Kind: symbol.KindSumVariant}
	okCall     = symbol.Identity{Lang: rust.Lang, Name: "Ok", Kind: symbol.KindSumVariant}
	boxFrom    = symbol.Identity{Lang: rust.Lang, Owner: boxName, Name: "from", Kind: symbol.KindMethod}
	boxNew     = symbol.Identity{Lang: rust.Lang, Owner: boxName, Name: "new", Kind: symbol.KindMethod}
	vecNew     = symbol.Identity{Lang: rust.Lang, Owner: vecName, Name: "new", Kind: symbol.KindMethod}
	vecFrom    = symbol.Identity{Lang: rust.Lang, Owner: vecName, Name: "from", Kind: symbol.KindMethod}
	arcFrom    = symbol.Identity{
		Lang:    rust.Lang,
		Package: "std/sync",
		Owner:   "Arc",
		Name:    "from",
		Kind:    symbol.KindMethod,
	}
	rcFrom   = symbol.Identity{Lang: rust.Lang, Package: "std/rc", Owner: "Rc", Name: "from", Kind: symbol.KindMethod}
	mutexNew = symbol.Identity{
		Lang:    rust.Lang,
		Package: "std/sync",
		Owner:   "Mutex",
		Name:    "new",
		Kind:    symbol.KindMethod,
	}
	pathNew = symbol.Identity{
		Lang:    rust.Lang,
		Package: "std/path",
		Owner:   "Path",
		Name:    "new",
		Kind:    symbol.KindMethod,
	}
	pathBufOf = symbol.Identity{
		Lang: rust.Lang, Package: "std/path", Owner: "PathBuf", Name: "from", Kind: symbol.KindMethod,
	}
	cowFrom = symbol.Identity{
		Lang:    rust.Lang,
		Package: "std/borrow",
		Owner:   "Cow",
		Name:    "from",
		Kind:    symbol.KindMethod,
	}
	setFrom = symbol.Identity{
		Lang: rust.Lang, Package: collectionsPkg, Owner: "HashSet", Name: "from", Kind: symbol.KindMethod,
	}
	dequeFrom = symbol.Identity{
		Lang: rust.Lang, Package: collectionsPkg, Owner: "VecDeque", Name: "from", Kind: symbol.KindMethod,
	}
	mapNew = symbol.Identity{
		Lang: rust.Lang, Package: collectionsPkg, Owner: "HashMap", Name: "new", Kind: symbol.KindMethod,
	}
	fromSecs = symbol.Identity{
		Lang: rust.Lang, Package: timePkg, Owner: durationName, Name: "from_secs", Kind: symbol.KindMethod,
	}
	durationZero = symbol.Identity{
		Lang: rust.Lang, Package: timePkg, Owner: durationName, Name: "ZERO", Kind: symbol.KindField,
	}
	unixEpoch = symbol.Identity{
		Lang: rust.Lang, Package: timePkg, Owner: "SystemTime", Name: "UNIX_EPOCH", Kind: symbol.KindField,
	}
	addCall = symbol.Identity{Lang: rust.Lang, Package: "std/ops", Owner: "Add", Name: "add", Kind: symbol.KindMethod}
	ioOther = symbol.Identity{
		Lang: rust.Lang, Package: "std", Owner: "io.Error", Name: "other", Kind: symbol.KindMethod,
	}
	fmtError = symbol.Identity{Lang: rust.Lang, Package: "std", Owner: "fmt", Name: errorName, Kind: symbol.KindStruct}
)

// The declarations of the fixture that the values name and call.
var (
	statusNew = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Status", Name: "New", Kind: symbol.KindEnumVariant,
	}
	statusPaid = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Status", Name: "Paid", Kind: symbol.KindEnumVariant,
	}
	colorRed = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Color", Name: "Red", Kind: symbol.KindEnumVariant,
	}
	colorGreen = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Color", Name: "Green", Kind: symbol.KindEnumVariant,
	}
	shapeEmpty = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Shape", Name: "Empty", Kind: symbol.KindSumVariant,
	}
	shapeCircle = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Shape", Name: "Circle", Kind: symbol.KindSumVariant,
	}
	treeLeaf = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Tree", Name: "Leaf", Kind: symbol.KindSumVariant,
	}
	orderNew = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Order", Name: "new", Kind: symbol.KindMethod,
	}
	storeErrNF = symbol.Identity{
		Lang: rust.Lang, Package: errorsPkg, Owner: "StoreError", Name: "NotFound", Kind: symbol.KindSumVariant,
	}
	storeErrIo = symbol.Identity{
		Lang: rust.Lang, Package: errorsPkg, Owner: "StoreError", Name: "Io", Kind: symbol.KindSumVariant,
	}
	linkID     = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Link", Kind: symbol.KindStruct}
	parseErrID = symbol.Identity{Lang: rust.Lang, Package: errorsPkg, Name: "ParseError", Kind: symbol.KindStruct}
	storeErrID = symbol.Identity{Lang: rust.Lang, Package: errorsPkg, Name: "StoreError", Kind: symbol.KindSum}
	singleID   = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Single", Kind: symbol.KindEnum}
	singleOnly = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Single", Name: "Only", Kind: symbol.KindEnumVariant,
	}
	neverID     = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Never", Kind: symbol.KindEnum}
	momentID    = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Moment", Kind: symbol.KindSum}
	momentUnset = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Moment", Name: "Unset", Kind: symbol.KindSumVariant,
	}
	clockID    = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Clock", Kind: symbol.KindSum}
	eventID    = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Event", Kind: symbol.KindSum}
	eventMoved = symbol.Identity{
		Lang: rust.Lang, Package: modelsPkg, Owner: "Event", Name: "Moved", Kind: symbol.KindSumVariant,
	}
	keyID   = symbol.Identity{Lang: rust.Lang, Package: storePkg, Owner: "Named", Name: "Key", Kind: symbol.KindAlias}
	stateID = symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "State", Kind: symbol.KindAlias}
	emptyID = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "EMPTY", Kind: symbol.KindConstant}
)

// The numbers and the composite types that the expected values contain.
var (
	n42u8  = emit.Number(emit.LiteralInt, "42", 8)
	n7u8   = emit.Number(emit.LiteralInt, "7", 8)
	n0u8   = emit.Number(emit.LiteralInt, "0", 8)
	n5u8   = emit.Number(emit.LiteralInt, "5", 8)
	n42u32 = emit.Number(emit.LiteralInt, "42", 32)
	n7u32  = emit.Number(emit.LiteralInt, "7", 32)
	n42i32 = emit.Number(emit.LiteralInt, "42", 32)
	n42u64 = emit.Number(emit.LiteralInt, "42", 64)
	n7u64  = emit.Number(emit.LiteralInt, "7", 64)
	f15    = emit.Number(emit.LiteralFloat, "1.5", 64)
	vec    = &emit.TypeRef{Spelling: vecName, Form: symbol.FormList}
	array1 = &emit.TypeRef{Spelling: "[_]", Form: symbol.FormArray, Length: 1}
	tuple  = &emit.TypeRef{Spelling: "(_)", Form: symbol.FormTuple}
	unit   = emit.Composite(&emit.TypeRef{Spelling: "()", Form: symbol.FormTuple})
	idType = &emit.TypeRef{Spelling: "Id", Target: idID}
)

// TestValues pins the samples, the zeros and the literals that the rules
// derive in Rust's grammar of expressions, and the values of the texts that an
// author states.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("DeriveSamples", func(t *testing.T) {
		t.Parallel()

		itemType := &emit.TypeRef{Spelling: "Item", Target: itemID}
		itemA := emit.Composite(itemType,
			emit.NamedField("sku", emit.Call(stringFrom, emit.Literal(emit.LiteralString, "sku-a"))),
			emit.NamedField("quantity", n42u32), emit.NamedField("price", f15))
		itemB := emit.Composite(itemType,
			emit.NamedField("sku", emit.Call(stringFrom, emit.Literal(emit.LiteralString, "sku-b"))),
			emit.NamedField("quantity", n42u32), emit.NamedField("price", f15))
		array4 := &emit.TypeRef{Spelling: "[_]", Form: symbol.FormArray, Length: 4}
		mapType := &emit.TypeRef{Spelling: "HashMap", Package: collectionsPkg}
		fields := []struct {
			name          string
			give          string
			wantSample    emit.Value
			wantAlternate emit.Value
		}{
			{
				name: "derives the literal of a tuple struct", give: "id",
				wantSample:    emit.Composite(idType, emit.Element(n42u64)),
				wantAlternate: emit.Composite(idType, emit.Element(n7u64)),
			},
			{
				name: "derives the first two variants of an enum", give: "status",
				wantSample: emit.Name(statusNew), wantAlternate: emit.Name(statusPaid),
			},
			{
				name: "derives a vector of the literal of a struct", give: "items",
				wantSample:    emit.Composite(vec, emit.Element(itemA)),
				wantAlternate: emit.Composite(vec, emit.Element(itemB)),
			},
			{
				name: "derives Some of each value of an option", give: "note",
				wantSample:    emit.Call(someCall, emit.Call(stringFrom, emit.Literal(emit.LiteralString, "note-a"))),
				wantAlternate: emit.Call(someCall, emit.Call(stringFrom, emit.Literal(emit.LiteralString, "note-b"))),
			},
			{
				name: "derives the from of an array of one element for a set", give: "tags",
				wantSample: emit.Call(setFrom, emit.Composite(array1,
					emit.Element(emit.Call(stringFrom, emit.Literal(emit.LiteralString, "tags-a"))))),
				wantAlternate: emit.Call(setFrom, emit.Composite(array1,
					emit.Element(emit.Call(stringFrom, emit.Literal(emit.LiteralString, "tags-b"))))),
			},
			{
				name: "derives a map of one entry that differs in the key", give: "totals",
				wantSample: emit.Composite(mapType,
					emit.KeyedEntry(emit.Call(stringFrom, emit.Literal(emit.LiteralString, "totals-a")), f15)),
				wantAlternate: emit.Composite(mapType,
					emit.KeyedEntry(emit.Call(stringFrom, emit.Literal(emit.LiteralString, "totals-b")), f15)),
			},
			{
				name: "derives the sum of the epoch and a number of seconds for SystemTime", give: "placed",
				wantSample: emit.Call(addCall, emit.Name(unixEpoch),
					emit.Call(fromSecs, emit.Number(emit.LiteralInt, "1700000000", 64))),
				wantAlternate: emit.Call(addCall, emit.Name(unixEpoch),
					emit.Call(fromSecs, emit.Number(emit.LiteralInt, "1700000001", 64))),
			},
			{
				name: "derives a number of seconds for Duration", give: "timeout",
				wantSample:    emit.Call(fromSecs, emit.Number(emit.LiteralInt, "1", 64)),
				wantAlternate: emit.Call(fromSecs, emit.Number(emit.LiteralInt, "2", 64)),
			},
			{
				name: "derives an element per position of an array", give: "code",
				wantSample: emit.Composite(array4, emit.Element(n42u8), emit.Element(n42u8), emit.Element(n42u8),
					emit.Element(n42u8)),
				wantAlternate: emit.Composite(array4, emit.Element(n7u8), emit.Element(n7u8), emit.Element(n7u8),
					emit.Element(n7u8)),
			},
			{
				name: "derives a tuple of the values of its members", give: "pair",
				wantSample: emit.Composite(tuple, emit.Element(emit.Number(emit.LiteralInt, "42", 32)),
					emit.Element(emit.Literal(emit.LiteralBool, "true"))),
				wantAlternate: emit.Composite(tuple, emit.Element(emit.Number(emit.LiteralInt, "7", 32)),
					emit.Element(emit.Literal(emit.LiteralBool, "false"))),
			},
			{
				name: "derives a string literal for a borrow of str", give: "label",
				wantSample:    emit.Literal(emit.LiteralString, "label-a"),
				wantAlternate: emit.Literal(emit.LiteralString, "label-b"),
			},
			{
				name: "derives a character literal for char", give: "initial",
				wantSample: emit.Raw(rust.Lang, "'a'"), wantAlternate: emit.Raw(rust.Lang, "'b'"),
			},
			{
				name: "derives the from of a string for an Arc of str", give: "shared",
				wantSample:    emit.Call(arcFrom, emit.Literal(emit.LiteralString, "shared-a")),
				wantAlternate: emit.Call(arcFrom, emit.Literal(emit.LiteralString, "shared-b")),
			},
		}
		for _, tt := range fields {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				field := f.field(t, orderID, tt.give)
				sample, alternate := rustrules.Rules{}.DeriveSamples(field.Type, field.Name, f.view)
				expect.Equal(t, sample, rules.Of(tt.wantSample), "the sample of "+tt.give)
				expect.Equal(t, alternate, rules.Of(tt.wantAlternate), "the alternate of "+tt.give)
			})
		}

		orderA := emit.Call(orderNew, emit.Composite(idType, emit.Element(n42u64)), emit.Name(statusNew))
		orderB := emit.Call(orderNew, emit.Composite(idType, emit.Element(n7u64)), emit.Name(statusNew))
		memoryType := &emit.TypeRef{Spelling: "Memory", Target: memoryID}
		pageType := &emit.TypeRef{Spelling: "Page", Target: pageID}
		guardedType := &emit.TypeRef{Spelling: "Guarded", Target: guardedID}
		declared := []struct {
			name          string
			give          *node.TypeRef
			wantSample    rules.Sample
			wantAlternate rules.Sample
		}{
			{
				name:          "derives the call of the first constructor of a struct",
				give:          &node.TypeRef{Spelling: "Order", Target: orderID},
				wantSample:    rules.Of(orderA),
				wantAlternate: rules.Of(orderB),
			},
			{
				name: "derives the literal of a struct after a constructor without parameters",
				give: &node.TypeRef{Spelling: "Memory", Target: memoryID},
				wantSample: rules.Of(
					emit.Composite(memoryType, emit.NamedField("orders", emit.Composite(vec, emit.Element(orderA)))),
				),
				wantAlternate: rules.Of(
					emit.Composite(memoryType, emit.NamedField("orders", emit.Composite(vec, emit.Element(orderB)))),
				),
			},
			{
				name: "derives the literal of a generic struct with its type arguments substituted",
				give: &node.TypeRef{Spelling: "Page", Target: pageID, Args: []*node.TypeRef{{Spelling: u8Name}}},
				wantSample: rules.Of(emit.Composite(pageType,
					emit.NamedField("items", emit.Composite(vec, emit.Element(n42u8))),
					emit.NamedField("next", emit.Call(someCall, n42u32)))),
				wantAlternate: rules.Of(emit.Composite(pageType,
					emit.NamedField("items", emit.Composite(vec, emit.Element(n7u8))),
					emit.NamedField("next", emit.Call(someCall, n42u32)))),
			},
			{
				name: "derives the new of a lock",
				give: &node.TypeRef{Spelling: "Guarded", Target: guardedID},
				wantSample: rules.Of(
					emit.Composite(guardedType, emit.NamedField("count", emit.Call(mutexNew, n42u32))),
				),
				wantAlternate: rules.Of(
					emit.Composite(guardedType, emit.NamedField("count", emit.Call(mutexNew, n7u32))),
				),
			},
			{
				name:          "derives one value for a unit struct",
				give:          &node.TypeRef{Spelling: "Nothing", Target: nothingID},
				wantSample:    rules.Of(emit.Composite(&emit.TypeRef{Spelling: "Nothing", Target: nothingID})),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "derives the first two variants of an enum",
				give:          &node.TypeRef{Spelling: "Color", Target: colorID},
				wantSample:    rules.Of(emit.Name(colorRed)),
				wantAlternate: rules.Of(emit.Name(colorGreen)),
			},
			{
				name:          "derives a unit variant and the next variant of a data enum",
				give:          &node.TypeRef{Spelling: "Shape", Target: shapeID},
				wantSample:    rules.Of(emit.Name(shapeEmpty)),
				wantAlternate: rules.Of(emit.Call(shapeCircle, f15)),
			},
			{
				name: "derives the two values of the field of a tuple variant",
				give: &node.TypeRef{
					Spelling: "Tree",
					Target:   treeID,
					Args:     []*node.TypeRef{{Spelling: u8Name}},
				},
				wantSample:    rules.Of(emit.Call(treeLeaf, n42u8)),
				wantAlternate: rules.Of(emit.Call(treeLeaf, n7u8)),
			},
			{
				name:       "derives an error of io for a variant of an error",
				give:       &node.TypeRef{Spelling: "StoreError", Target: storeErrID},
				wantSample: rules.Of(emit.Name(storeErrNF)),
				wantAlternate: rules.Of(emit.Call(storeErrIo,
					emit.Call(ioOther, emit.Literal(emit.LiteralString, "error-a")))),
			},
			{
				name:          "derives the pair of the target of an alias",
				give:          &node.TypeRef{Spelling: "Ids", Target: idsID},
				wantSample:    rules.Of(emit.Composite(vec, emit.Element(n42u64))),
				wantAlternate: rules.Of(emit.Composite(vec, emit.Element(n7u64))),
			},
			{
				name:          "derives one value for an enum of one variant",
				give:          &node.TypeRef{Spelling: "Single", Target: singleID},
				wantSample:    rules.Of(emit.Name(singleOnly)),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses an enum without variants",
				give:          &node.TypeRef{Spelling: "Never", Target: neverID},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "derives the variant without fields where no other variant derives",
				give:          &node.TypeRef{Spelling: "Moment", Target: momentID},
				wantSample:    rules.Of(emit.Name(momentUnset)),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a data enum whose variants derive no value",
				give:          &node.TypeRef{Spelling: "Clock", Target: clockID},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name: "derives the literal of a struct variant",
				give: &node.TypeRef{Spelling: "Event", Target: eventID},
				wantSample: rules.Of(emit.Composite(&emit.TypeRef{Spelling: "Event::Moved", Target: eventMoved},
					emit.NamedField("x", n42i32), emit.NamedField("y", n42i32))),
				wantAlternate: rules.Of(emit.Composite(&emit.TypeRef{Spelling: "Event::Moved", Target: eventMoved},
					emit.NamedField("x", emit.Number(emit.LiteralInt, "7", 32)), emit.NamedField("y", n42i32))),
			},
			{
				name:          "refuses an associated type without a target",
				give:          &node.TypeRef{Spelling: "Key", Target: keyID},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a struct with a private field and no constructor",
				give:          &node.TypeRef{Spelling: "Settings", Target: settingsID},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a struct with a field of a type of the standard library without a value",
				give:          &node.TypeRef{Spelling: "ParseError", Target: parseErrID},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a struct that refers to itself past the budget of the walk",
				give:          &node.TypeRef{Spelling: "Link", Target: linkID},
				wantSample:    rules.Refused(rules.RefusedDepth),
				wantAlternate: rules.Refused(rules.RefusedDepth),
			},
			{
				name:          "refuses a trait",
				give:          &node.TypeRef{Spelling: "Store", Target: storeID},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a declaration outside the view",
				give:          &node.TypeRef{Spelling: "Gone", Target: goneID},
				wantSample:    rules.Refused(rules.RefusedUnresolved),
				wantAlternate: rules.Refused(rules.RefusedUnresolved),
			},
		}
		for _, tt := range declared {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				sample, alternate := rustrules.Rules{}.DeriveSamples(tt.give, "error", f.view)
				expect.Equal(t, sample, tt.wantSample, "the sample of "+tt.give.Spelling)
				expect.Equal(t, alternate, tt.wantAlternate, "the alternate of "+tt.give.Spelling)
			})
		}

		t.Run("refuses a type parameter", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			items := f.field(t, pageID, "items").Type
			sample, _ := rustrules.Rules{}.DeriveSamples(items.Args[0], "", f.view)
			expect.Equal(t, sample, rules.Refused(rules.RefusedNoLiteral), "a type parameter has no value")
		})

		t.Run("takes the values that an author stated for a field", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			quantity := f.field(t, itemID, "quantity")
			claim := meta.Claim{Subject: quantity.ID, Authority: meta.AuthorityPlugin, Plugin: "fixture"}
			for key, value := range map[meta.Key[string]]string{f.Keys.Sample: "5", f.Keys.Alternate: "6"} {
				assert.NoError(t, meta.Stamp(f.Facts, key, value, claim), "the fixture stamp applies")
			}
			sample, _ := rustrules.Rules{}.DeriveSamples(&node.TypeRef{Spelling: "Item", Target: itemID}, "", f.view)
			assert.True(t, sample.OK(), "the sample derives")
			assert.Length(t, sample.Value.Fields, 3, "the literal has a value for each field")
			expect.Equal(t, sample.Value.Fields[1], emit.NamedField("quantity", emit.Number(emit.LiteralInt, "5", 32)),
				"the sample takes the stated value")
		})

		t.Run("derives the half that an author did not state", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			sku := f.field(t, itemID, "sku")
			claim := meta.Claim{Subject: sku.ID, Authority: meta.AuthorityPlugin, Plugin: "fixture"}
			assert.NoError(t, meta.Stamp(f.Facts, f.Keys.Sample, "stated", claim), "the fixture stamp applies")
			_, alternate := rustrules.Rules{}.DeriveSamples(&node.TypeRef{Spelling: "Item", Target: itemID}, "", f.view)
			assert.True(t, alternate.OK(), "the alternate derives")
			assert.NotEmpty(t, alternate.Value.Fields, "the literal has values")
			expect.Equal(t, alternate.Value.Fields[0],
				emit.NamedField("sku", emit.Call(stringFrom, emit.Literal(emit.LiteralString, "sku-b"))),
				"the alternate is the derived value")
		})

		u8 := &node.TypeRef{Spelling: u8Name}
		str := &node.TypeRef{Spelling: strName}
		path := &node.TypeRef{Spelling: "Path", Package: "std/path"}
		slice := &node.TypeRef{Spelling: "[u8]", Form: symbol.FormList, Elems: []*node.TypeRef{u8}}
		dynError := &node.TypeRef{
			Spelling: "dyn Error", Form: symbol.FormIntersection,
			Elems: []*node.TypeRef{{Spelling: errorName, Package: "std/error"}},
		}
		valueA, valueB := emit.Literal(emit.LiteralString, "value-a"), emit.Literal(emit.LiteralString, "value-b")
		refs := []struct {
			name          string
			give          *node.TypeRef
			wantSample    rules.Sample
			wantAlternate rules.Sample
		}{
			{
				name:          "derives one value for the unit type",
				give:          &node.TypeRef{Spelling: "()"},
				wantSample:    rules.Of(unit),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "derives the address of each value of a shared borrow",
				give:          &node.TypeRef{Spelling: "&u8", Form: symbol.FormBorrow, Elems: []*node.TypeRef{u8}},
				wantSample:    rules.Of(emit.Address(n42u8)),
				wantAlternate: rules.Of(emit.Address(n7u8)),
			},
			{
				name:          "derives the address of an array for a borrow of a slice",
				give:          &node.TypeRef{Spelling: "&[u8]", Form: symbol.FormBorrow, Elems: []*node.TypeRef{slice}},
				wantSample:    rules.Of(emit.Address(emit.Composite(array1, emit.Element(n42u8)))),
				wantAlternate: rules.Of(emit.Address(emit.Composite(array1, emit.Element(n7u8)))),
			},
			{
				name:          "derives the new of a string for a borrow of Path",
				give:          &node.TypeRef{Spelling: "&Path", Form: symbol.FormBorrow, Elems: []*node.TypeRef{path}},
				wantSample:    rules.Of(emit.Call(pathNew, valueA)),
				wantAlternate: rules.Of(emit.Call(pathNew, valueB)),
			},
			{
				name:          "derives the from of a string for a Box of str",
				give:          &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{str}},
				wantSample:    rules.Of(emit.Call(boxFrom, valueA)),
				wantAlternate: rules.Of(emit.Call(boxFrom, valueB)),
			},
			{
				name:          "derives the from of an array for a Box of a slice",
				give:          &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{slice}},
				wantSample:    rules.Of(emit.Call(boxFrom, emit.Composite(array1, emit.Element(n42u8)))),
				wantAlternate: rules.Of(emit.Call(boxFrom, emit.Composite(array1, emit.Element(n7u8)))),
			},
			{
				name:          "derives the new of each value for a Box of a type with a size",
				give:          &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{u8}},
				wantSample:    rules.Of(emit.Call(boxNew, n42u8)),
				wantAlternate: rules.Of(emit.Call(boxNew, n7u8)),
			},
			{
				name:          "derives the from of a Path for an Rc of Path",
				give:          &node.TypeRef{Spelling: "Rc", Package: "std/rc", Args: []*node.TypeRef{path}},
				wantSample:    rules.Of(emit.Call(rcFrom, emit.Call(pathNew, valueA))),
				wantAlternate: rules.Of(emit.Call(rcFrom, emit.Call(pathNew, valueB))),
			},
			{
				name:          "derives the from of a string for a Box of a trait object of the error trait",
				give:          &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{dynError}},
				wantSample:    rules.Of(emit.Call(boxFrom, valueA)),
				wantAlternate: rules.Of(emit.Call(boxFrom, valueB)),
			},
			{
				name:          "derives the other of a string for io::Error",
				give:          &node.TypeRef{Spelling: "io::Error", Package: "std/io"},
				wantSample:    rules.Of(emit.Call(ioOther, valueA)),
				wantAlternate: rules.Of(emit.Call(ioOther, valueB)),
			},
			{
				name:          "derives one value for fmt::Error",
				give:          &node.TypeRef{Spelling: "fmt::Error", Package: "std/fmt"},
				wantSample:    rules.Of(emit.Name(fmtError)),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "derives the from of a string for Cow of str",
				give:          &node.TypeRef{Spelling: "Cow", Package: "std/borrow", Args: []*node.TypeRef{str}},
				wantSample:    rules.Of(emit.Call(cowFrom, valueA)),
				wantAlternate: rules.Of(emit.Call(cowFrom, valueB)),
			},
			{
				name:          "derives the from of a string for PathBuf",
				give:          &node.TypeRef{Spelling: "PathBuf", Package: "std/path"},
				wantSample:    rules.Of(emit.Call(pathBufOf, valueA)),
				wantAlternate: rules.Of(emit.Call(pathBufOf, valueB)),
			},
			{
				name:          "derives Ok of each value of a result",
				give:          &node.TypeRef{Spelling: resultName, Args: []*node.TypeRef{u8, {Spelling: stringName}}},
				wantSample:    rules.Of(emit.Call(okCall, n42u8)),
				wantAlternate: rules.Of(emit.Call(okCall, n7u8)),
			},
			{
				name: "derives one value for a result of the unit type",
				give: &node.TypeRef{
					Spelling: resultName,
					Args:     []*node.TypeRef{{Spelling: "()"}, {Spelling: stringName}},
				},
				wantSample:    rules.Of(emit.Call(okCall, unit)),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "derives the from of an array for VecDeque",
				give:          &node.TypeRef{Spelling: "VecDeque", Package: collectionsPkg, Args: []*node.TypeRef{u8}},
				wantSample:    rules.Of(emit.Call(dequeFrom, emit.Composite(array1, emit.Element(n42u8)))),
				wantAlternate: rules.Of(emit.Call(dequeFrom, emit.Composite(array1, emit.Element(n7u8)))),
			},
		}
		for _, tt := range refs {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				sample, alternate := rustrules.Rules{}.DeriveSamples(tt.give, "value", rules.View{})
				expect.Equal(t, sample, tt.wantSample, "the sample of "+tt.give.Spelling)
				expect.Equal(t, alternate, tt.wantAlternate, "the alternate of "+tt.give.Spelling)
			})
		}

		never := &node.TypeRef{Spelling: "!"}
		refusals := []struct {
			name string
			give *node.TypeRef
			want rules.Refusal
		}{
			{name: "refuses a nil reference", want: rules.RefusedNoLiteral},
			{name: "refuses the never type", give: never, want: rules.RefusedNoLiteral},
			{
				name: "refuses a borrow without its type", give: &node.TypeRef{Spelling: "&", Form: symbol.FormBorrow},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a mutable borrow",
				give: &node.TypeRef{
					Spelling: "&mut u8", Form: symbol.FormBorrow, Mutability: symbol.MutabilityMutable,
					Elems: []*node.TypeRef{u8},
				},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses the error trait", give: &node.TypeRef{Spelling: errorName, Package: "std/error"},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses CString", give: &node.TypeRef{Spelling: "CString", Package: "std/ffi"},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses Instant", give: &node.TypeRef{Spelling: "Instant", Package: timePkg},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses an iterator",
				give: &node.TypeRef{Spelling: iteratorName, Args: []*node.TypeRef{
					{Spelling: "Item=u8", Args: []*node.TypeRef{u8}},
				}},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a type of the standard library outside the table",
				give: &node.TypeRef{Spelling: "IpAddr", Package: "std/net"}, want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a type of another crate", give: &node.TypeRef{Spelling: "Utc", Package: "chrono"},
				want: rules.RefusedUnresolved,
			},
			{
				name: "refuses a name that nothing binds",
				give: &node.TypeRef{Spelling: "Thing"},
				want: rules.RefusedUnresolved,
			},
			{
				name: "refuses an array without elements",
				give: &node.TypeRef{Spelling: "[u8;0]", Form: symbol.FormArray, Elems: []*node.TypeRef{u8}},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses an array longer than the rules write",
				give: &node.TypeRef{
					Spelling: "[u8;65]",
					Form:     symbol.FormArray,
					Length:   65,
					Elems:    []*node.TypeRef{u8},
				},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses an array of a type without a value",
				give: &node.TypeRef{
					Spelling: "[!;1]",
					Form:     symbol.FormArray,
					Length:   1,
					Elems:    []*node.TypeRef{never},
				},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a tuple without members", give: &node.TypeRef{Spelling: "(,)", Form: symbol.FormTuple},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a tuple of a member without a value",
				give: &node.TypeRef{Spelling: "(u8,Utc)", Form: symbol.FormTuple, Elems: []*node.TypeRef{
					u8, {Spelling: "Utc", Package: "chrono"},
				}},
				want: rules.RefusedUnresolved,
			},
			{
				name: "refuses a map of a key without a value",
				give: &node.TypeRef{Spelling: "HashMap", Package: collectionsPkg, Args: []*node.TypeRef{never, u8}},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a result of a value without a value",
				give: &node.TypeRef{Spelling: resultName, Args: []*node.TypeRef{never, u8}},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a function", give: &node.TypeRef{Spelling: "fn()", Form: symbol.FormFunc},
				want: rules.RefusedNoLiteral,
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				sample, alternate := rustrules.Rules{}.DeriveSamples(tt.give, "value", rules.View{})
				expect.Equal(t, sample, rules.Refused(tt.want), "the sample refuses with its reason")
				expect.Equal(t, alternate, rules.Refused(tt.want), "the alternate refuses with its reason")
			})
		}
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		u8 := &node.TypeRef{Spelling: u8Name}
		str := &node.TypeRef{Spelling: strName}
		never := &node.TypeRef{Spelling: "!"}
		slice := &node.TypeRef{Spelling: "[u8]", Form: symbol.FormList, Elems: []*node.TypeRef{u8}}
		empty := emit.Composite(&emit.TypeRef{Spelling: "[_]", Form: symbol.FormArray})
		tests := []struct {
			name string
			give *node.TypeRef
			want emit.Value
		}{
			{name: "returns 0 for an integer", give: u8, want: n0u8},
			{
				name: "returns 0 for a float", give: &node.TypeRef{Spelling: "f32"},
				want: emit.Number(emit.LiteralFloat, "0", 32),
			},
			{
				name: "returns false for bool", give: &node.TypeRef{Spelling: "bool"},
				want: emit.Literal(emit.LiteralBool, "false"),
			},
			{
				name: "returns the nul character for char", give: &node.TypeRef{Spelling: charName},
				want: emit.Raw(rust.Lang, `'\0'`),
			},
			{
				name: "returns the empty string for a borrow of str",
				give: &node.TypeRef{Spelling: "&str", Form: symbol.FormBorrow, Elems: []*node.TypeRef{str}},
				want: emit.Literal(emit.LiteralString, ""),
			},
			{name: "returns the new of String", give: &node.TypeRef{Spelling: stringName}, want: emit.Call(stringNew)},
			{
				name: "returns the new of the empty string for a borrow of Path",
				give: &node.TypeRef{Spelling: "&Path", Form: symbol.FormBorrow, Elems: []*node.TypeRef{
					{Spelling: "Path", Package: "std/path"},
				}},
				want: emit.Call(pathNew, emit.Literal(emit.LiteralString, "")),
			},
			{
				name: "returns the from of the empty string for Cow of str",
				give: &node.TypeRef{Spelling: "Cow", Package: "std/borrow", Args: []*node.TypeRef{str}},
				want: emit.Call(cowFrom, emit.Literal(emit.LiteralString, "")),
			},
			{name: "returns the unit value for the unit type", give: &node.TypeRef{Spelling: "()"}, want: unit},
			{
				name: "returns None for an option",
				give: &node.TypeRef{
					Spelling: optionName,
					Args:     []*node.TypeRef{u8},
				},
				want: emit.Literal(emit.LiteralNil, ""),
			},
			{
				name: "returns the new of Vec", give: &node.TypeRef{Spelling: vecName, Args: []*node.TypeRef{u8}},
				want: emit.Call(vecNew),
			},
			{
				name: "returns the new of HashMap",
				give: &node.TypeRef{Spelling: "HashMap", Package: collectionsPkg, Args: []*node.TypeRef{u8, u8}},
				want: emit.Call(mapNew),
			},
			{
				name: "returns an array of zeros",
				give: &node.TypeRef{Spelling: "[u8;2]", Form: symbol.FormArray, Length: 2, Elems: []*node.TypeRef{u8}},
				want: emit.Composite(&emit.TypeRef{Spelling: "[_]", Form: symbol.FormArray, Length: 2},
					emit.Element(n0u8), emit.Element(n0u8)),
			},
			{
				name: "returns the address of the empty array for a borrow of a slice",
				give: &node.TypeRef{Spelling: "&[u8]", Form: symbol.FormBorrow, Elems: []*node.TypeRef{slice}},
				want: emit.Address(empty),
			},
			{
				name: "returns a tuple of zeros",
				give: &node.TypeRef{Spelling: "(u8,bool)", Form: symbol.FormTuple, Elems: []*node.TypeRef{
					u8, {Spelling: "bool"},
				}},
				want: emit.Composite(tuple, emit.Element(n0u8), emit.Element(emit.Literal(emit.LiteralBool, "false"))),
			},
			{
				name: "returns the new of the zero of its type for a Box",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{u8}}, want: emit.Call(boxNew, n0u8),
			},
			{
				name: "returns the from of the empty array for a Box of a slice",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{slice}}, want: emit.Call(boxFrom, empty),
			},
			{
				name: "returns ZERO for Duration",
				give: &node.TypeRef{Spelling: durationName, Package: timePkg}, want: emit.Name(durationZero),
			},
			{
				name: "returns UNIX_EPOCH for SystemTime",
				give: &node.TypeRef{Spelling: "SystemTime", Package: timePkg}, want: emit.Name(unixEpoch),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := rustrules.Rules{}.ZeroValue(tt.give, rules.View{})
				assert.True(t, ok, "the type has a zero value")
				assert.Equal(t, got, tt.want, "the zero value of "+tt.give.Spelling)
			})
		}

		declared := []struct {
			name string
			give *node.TypeRef
			want emit.Value
		}{
			{
				name: "returns the variant that #[default] marks for an enum that derives Default",
				give: &node.TypeRef{Spelling: "Status", Target: statusID}, want: emit.Name(statusNew),
			},
			{
				name: "returns default for a struct that derives Default",
				give: &node.TypeRef{Spelling: "Settings", Target: settingsID},
				want: emit.Call(symbol.Identity{
					Lang: rust.Lang, Package: modelsPkg, Owner: "Settings", Name: "default", Kind: symbol.KindMethod,
				}),
			},
			{
				name: "returns default for an enum that a trait impl implements Default for",
				give: &node.TypeRef{Spelling: "Grade", Target: gradeID},
				want: emit.Call(symbol.Identity{
					Lang: rust.Lang, Package: modelsPkg, Owner: "Grade", Name: "default", Kind: symbol.KindMethod,
				}),
			},
			{
				name: "returns default for a data enum that derives Default",
				give: &node.TypeRef{Spelling: "Shape", Target: shapeID},
				want: emit.Call(symbol.Identity{
					Lang: rust.Lang, Package: modelsPkg, Owner: "Shape", Name: "default", Kind: symbol.KindMethod,
				}),
			},
			{
				name: "returns default for a data enum that a trait impl implements Default for",
				give: &node.TypeRef{Spelling: "Reading", Target: readingID},
				want: emit.Call(symbol.Identity{
					Lang: rust.Lang, Package: modelsPkg, Owner: "Reading", Name: "default", Kind: symbol.KindMethod,
				}),
			},
			{
				name: "returns the zero of the target of an alias",
				give: &node.TypeRef{Spelling: "Ids", Target: idsID}, want: emit.Call(vecNew),
			},
		}
		for _, tt := range declared {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, ok := rustrules.Rules{}.ZeroValue(tt.give, f.view)
				assert.True(t, ok, "the type has a zero value")
				assert.Equal(t, got, tt.want, "the zero value of "+tt.give.Spelling)
			})
		}

		none := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "reports false for a nil reference"},
			{
				name: "reports false for a mutable borrow",
				give: &node.TypeRef{
					Spelling: "&mut str", Form: symbol.FormBorrow, Mutability: symbol.MutabilityMutable,
					Elems: []*node.TypeRef{str},
				},
			},
			{
				name: "reports false for a borrow of a type with a size",
				give: &node.TypeRef{Spelling: "&u8", Form: symbol.FormBorrow, Elems: []*node.TypeRef{u8}},
			},
			{
				name: "reports false for a borrow of a text without a zero",
				give: &node.TypeRef{Spelling: "&CStr", Form: symbol.FormBorrow, Elems: []*node.TypeRef{
					{Spelling: "CStr", Package: "std/ffi"},
				}},
			},
			{
				name: "reports false for an enum without a default",
				give: &node.TypeRef{Spelling: "Color", Target: colorID},
			},
			{
				name: "reports false for a struct without Default",
				give: &node.TypeRef{Spelling: "Order", Target: orderID},
			},
			{name: "reports false for a trait", give: &node.TypeRef{Spelling: "Store", Target: storeID}},
			{
				name: "reports false for a declaration outside the view",
				give: &node.TypeRef{Spelling: "Gone", Target: goneID},
			},
			{
				name: "reports false for a result",
				give: &node.TypeRef{Spelling: resultName, Args: []*node.TypeRef{u8, u8}},
			},
			{name: "reports false for io::Error", give: &node.TypeRef{Spelling: "io::Error", Package: "std/io"}},
			{name: "reports false for CString", give: &node.TypeRef{Spelling: "CString", Package: "std/ffi"}},
			{
				name: "reports false for a type of another crate",
				give: &node.TypeRef{Spelling: "Utc", Package: "chrono"},
			},
			{
				name: "reports false for a Box of a trait object of the error trait",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{{
					Spelling: "dyn Error", Form: symbol.FormIntersection, Elems: []*node.TypeRef{{Spelling: errorName}},
				}}},
			},
			{
				name: "reports false for a Box of a type without a zero",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{never}},
			},
			{
				name: "reports false for an array longer than the rules write",
				give: &node.TypeRef{
					Spelling: "[u8;65]",
					Form:     symbol.FormArray,
					Length:   65,
					Elems:    []*node.TypeRef{u8},
				},
			},
			{
				name: "reports false for an array of a type without a zero",
				give: &node.TypeRef{
					Spelling: "[!;1]",
					Form:     symbol.FormArray,
					Length:   1,
					Elems:    []*node.TypeRef{never},
				},
			},
			{
				name: "reports false for a tuple of a member without a zero",
				give: &node.TypeRef{Spelling: "(!,)", Form: symbol.FormTuple, Elems: []*node.TypeRef{never}},
			},
			{name: "reports false for a function", give: &node.TypeRef{Spelling: "fn()", Form: symbol.FormFunc}},
		}
		for _, tt := range none {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, ok := rustrules.Rules{}.ZeroValue(tt.give, f.view)
				assert.False(t, ok, "the type has no zero value")
			})
		}
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		nestedOption := &node.TypeRef{Spelling: u8Name}
		for range 10 {
			nestedOption = &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{nestedOption}}
		}
		u8 := &node.TypeRef{Spelling: u8Name}
		str := &node.TypeRef{Spelling: strName}
		slice := &node.TypeRef{Spelling: "[u8]", Form: symbol.FormList, Elems: []*node.TypeRef{u8}}
		letterA := emit.Literal(emit.LiteralString, "a")
		tests := []struct {
			name string
			give *node.TypeRef
			text string
			want emit.Value
		}{
			{
				name: "reads an integer of the range of its type",
				give: u8,
				text: "255",
				want: emit.Number(emit.LiteralInt, "255", 8),
			},
			{name: "reads an integer with the suffix of its type", give: u8, text: "5u8", want: n5u8},
			{
				name: "reads a negative integer of a signed type", give: &node.TypeRef{Spelling: "i64"}, text: "-5",
				want: emit.Number(emit.LiteralInt, "-5", 64),
			},
			{
				name: "reads an integer wider than 64 bits",
				give: &node.TypeRef{Spelling: "u128"},
				text: "18446744073709551616",
				want: emit.Number(emit.LiteralInt, "18446744073709551616", 128),
			},
			{
				name: "reads a float", give: &node.TypeRef{Spelling: "f64"}, text: "2.5",
				want: emit.Number(emit.LiteralFloat, "2.5", 64),
			},
			{
				name: "reads an integer as a float", give: &node.TypeRef{Spelling: "f32"}, text: "3",
				want: emit.Number(emit.LiteralFloat, "3", 32),
			},
			{
				name: "reads true for bool", give: &node.TypeRef{Spelling: "bool"}, text: "true",
				want: emit.Literal(emit.LiteralBool, "true"),
			},
			{
				name: "reads a character for char", give: &node.TypeRef{Spelling: charName}, text: "'x'",
				want: emit.Raw(rust.Lang, "'x'"),
			},
			{name: "reads a string for str", give: str, text: `"a"`, want: letterA},
			{
				name: "reads a string for a borrow of str",
				give: &node.TypeRef{
					Spelling: "&str",
					Form:     symbol.FormBorrow,
					Elems:    []*node.TypeRef{str},
				},
				text: `"a"`,
				want: letterA,
			},
			{
				name: "reads the from of a string for String", give: &node.TypeRef{Spelling: stringName}, text: `"a"`,
				want: emit.Call(stringFrom, letterA),
			},
			{
				name: "reads the new of a string for Path",
				give: &node.TypeRef{
					Spelling: "Path",
					Package:  "std/path",
				},
				text: `r"a"`,
				want: emit.Call(pathNew, letterA),
			},
			{
				name: "reads a byte string for a borrow of a slice of u8",
				give: &node.TypeRef{
					Spelling: "&[u8]",
					Form:     symbol.FormBorrow,
					Elems:    []*node.TypeRef{slice},
				},
				text: `b"a"`,
				want: emit.Raw(rust.Lang, `b"a"`),
			},
			{
				name: "reads the from of a byte string for Vec of u8",
				give: &node.TypeRef{Spelling: vecName, Args: []*node.TypeRef{u8}}, text: `b"a"`,
				want: emit.Call(vecFrom, emit.Raw(rust.Lang, `b"a"`)),
			},
			{
				name: "reads None for an option",
				give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{u8}}, text: "None",
				want: emit.Literal(emit.LiteralNil, ""),
			},
			{
				name: "reads Some of a value of its type for an option",
				give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{u8}}, text: "Some(5)",
				want: emit.Call(someCall, n5u8),
			},
			{
				name: "reads the address of a value for a shared borrow",
				give: &node.TypeRef{Spelling: "&u8", Form: symbol.FormBorrow, Elems: []*node.TypeRef{u8}}, text: "5",
				want: emit.Address(n5u8),
			},
			{
				name: "reads the new of a value for a Box",
				give: &node.TypeRef{
					Spelling: boxName,
					Args:     []*node.TypeRef{u8},
				},
				text: "5",
				want: emit.Call(boxNew, n5u8),
			},
			{
				name: "reads the from of a string for a Box of str",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{str}}, text: `"a"`,
				want: emit.Call(boxFrom, letterA),
			},
			{
				name: "reads the unit value for the unit type",
				give: &node.TypeRef{Spelling: "()"},
				text: "()",
				want: unit,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, read := rustrules.Rules{}.LiteralFor(rules.Scope{}, tt.give, tt.text, rules.View{})
				assert.True(t, read, "the text is a value of the type")
				assert.Equal(t, got, tt.want, "the value of "+tt.text)
			})
		}

		declared := []struct {
			name    string
			subject symbol.Identity
			give    *node.TypeRef
			text    string
			want    emit.Value
		}{
			{
				name: "reads a variant of an enum by its name", subject: storeID,
				give: &node.TypeRef{Spelling: "Status", Target: statusID}, text: "Paid", want: emit.Name(statusPaid),
			},
			{
				name: "reads a variant of an enum by its path", subject: storeID,
				give: &node.TypeRef{Spelling: "Status", Target: statusID}, text: "models::Status::Paid",
				want: emit.Name(statusPaid),
			},
			{
				name:    "reads a unit variant of a data enum",
				subject: storeID,
				give: &node.TypeRef{
					Spelling: "Shape",
					Target:   shapeID,
				},
				text: "Shape::Empty",
				want: emit.Name(shapeEmpty),
			},
			{
				name: "reads a constant of the scope of its type", subject: belowID,
				give: &node.TypeRef{Spelling: "u32"}, text: "MAX_RETRIES", want: emit.Name(maxID),
			},
			{
				name: "reads a variant through an alias of its enum", subject: storeID,
				give: &node.TypeRef{Spelling: "State", Target: stateID}, text: "Paid", want: emit.Name(statusPaid),
			},
			{
				name: "reads a static of the scope of its type", subject: belowID,
				give: &node.TypeRef{
					Spelling: "&str", Form: symbol.FormBorrow, Mutability: symbol.MutabilityImmutable,
					Elems: []*node.TypeRef{{Spelling: strName}},
				},
				text: "GREETING", want: emit.Name(greetID),
			},
			{
				name: "reads a constant of a generic type", subject: belowID,
				give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{{Spelling: u8Name}}},
				text: "EMPTY", want: emit.Name(emptyID),
			},
		}
		for _, tt := range declared {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, read := rustrules.Rules{}.LiteralFor(f.view.ScopeOf(tt.subject), tt.give, tt.text, f.view)
				assert.True(t, read, "the text is a value of the type")
				assert.Equal(t, got, tt.want, "the value of "+tt.text)
			})
		}

		refusals := []struct {
			name string
			give *node.TypeRef
			text string
		}{
			{name: "refuses an integer beyond the range of its type", give: u8, text: "256"},
			{name: "refuses a negative integer of an unsigned type", give: u8, text: "-1"},
			{name: "refuses an integer with the suffix of another type", give: u8, text: "5u16"},
			{name: "refuses a float for an integer", give: u8, text: "1.5"},
			{
				name: "refuses a float beyond the range of a single precision",
				give: &node.TypeRef{Spelling: "f32"},
				text: "1e39",
			},
			{
				name: "refuses an integer of another radix for a float",
				give: &node.TypeRef{Spelling: "f64"},
				text: "0x10",
			},
			{name: "refuses the digits of another base for an integer", give: u8, text: "0b2"},
			{name: "refuses a string for an integer", give: u8, text: `"a"`},
			{name: "refuses an integer for String", give: &node.TypeRef{Spelling: stringName}, text: "5"},
			{name: "refuses a string for char", give: &node.TypeRef{Spelling: charName}, text: `"a"`},
			{
				name: "refuses a string for CString",
				give: &node.TypeRef{Spelling: "CString", Package: "std/ffi"},
				text: `"a"`,
			},
			{name: "refuses a text that is no literal", give: u8, text: "1 + 1"},
			{
				name: "refuses a type of another crate",
				give: &node.TypeRef{Spelling: "Utc", Package: "chrono"},
				text: "5",
			},
			{
				name: "refuses Some of a text that is no literal",
				give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{u8}}, text: "Some(a b)",
			},
			{
				name: "refuses a byte string for a borrow of a slice of another type",
				give: &node.TypeRef{Spelling: "&[Order]", Form: symbol.FormBorrow, Elems: []*node.TypeRef{
					{
						Spelling: "[Order]",
						Form:     symbol.FormList,
						Elems:    []*node.TypeRef{{Spelling: "Order", Target: orderID}},
					},
				}},
				text: `b"a"`,
			},
			{
				name: "refuses a literal nested deeper than the rules follow",
				give: nestedOption, text: "Some(Some(Some(Some(Some(Some(Some(Some(Some(Some(1))))))))))",
			},
			{
				name: "refuses Some of a value of another type",
				give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{u8}}, text: "Some(true)",
			},
			{
				name: "refuses a value for a mutable borrow",
				give: &node.TypeRef{
					Spelling: "&mut u8", Form: symbol.FormBorrow, Mutability: symbol.MutabilityMutable,
					Elems: []*node.TypeRef{u8},
				},
				text: "5",
			},
			{
				name: "refuses a string for a borrow of a slice",
				give: &node.TypeRef{
					Spelling: "&[u8]",
					Form:     symbol.FormBorrow,
					Elems:    []*node.TypeRef{slice},
				},
				text: `"a"`,
			},
			{
				name: "refuses a value of another type for a shared borrow",
				give: &node.TypeRef{Spelling: "&u8", Form: symbol.FormBorrow, Elems: []*node.TypeRef{u8}}, text: "true",
			},
			{
				name: "refuses a value of another type for a Box",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{u8}}, text: "true",
			},
			{
				name: "refuses a value for a tuple",
				give: &node.TypeRef{Spelling: "(u8,)", Form: symbol.FormTuple},
				text: "5",
			},
			{name: "refuses another value for the unit type", give: &node.TypeRef{Spelling: "()"}, text: "5"},
			{name: "refuses None for a type that is no option", give: u8, text: "None"},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, read := rustrules.Rules{}.LiteralFor(rules.Scope{}, tt.give, tt.text, rules.View{})
				assert.False(t, read, "the text is no value of the type")
			})
		}

		scoped := []struct {
			name    string
			subject symbol.Identity
			give    *node.TypeRef
			text    string
		}{
			{
				name: "refuses a variant of another enum", subject: storeID,
				give: &node.TypeRef{Spelling: "Status", Target: statusID}, text: "Color::Red",
			},
			{
				name: "refuses a variant of a data enum with fields", subject: storeID,
				give: &node.TypeRef{Spelling: "Shape", Target: shapeID}, text: "Circle",
			},
			{
				name: "refuses a constant of another type", subject: belowID,
				give: &node.TypeRef{Spelling: u8Name}, text: "MAX_RETRIES",
			},
			{
				name: "refuses a name that the scope does not declare", subject: belowID,
				give: &node.TypeRef{Spelling: "u32"}, text: "MISSING",
			},
			{
				name: "refuses a value of the target of an alias that the target does not take", subject: belowID,
				give: &node.TypeRef{Spelling: "Ids", Target: idsID}, text: "5",
			},
			{
				name: "refuses a path for an alias whose target has no variant", subject: belowID,
				give: &node.TypeRef{Spelling: "Ids", Target: idsID}, text: "Red",
			},
			{
				name: "refuses a value for a struct", subject: belowID,
				give: &node.TypeRef{Spelling: "Order", Target: orderID}, text: "5",
			},
			{
				name: "refuses a constant of another argument of a generic type", subject: belowID,
				give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{{Spelling: "u16"}}}, text: "EMPTY",
			},
			{
				name: "refuses a static for a type of another child", subject: belowID,
				give: &node.TypeRef{
					Spelling: "&", Form: symbol.FormBorrow, Mutability: symbol.MutabilityImmutable,
					Elems: []*node.TypeRef{nil},
				},
				text: "GREETING",
			},
		}
		for _, tt := range scoped {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, read := rustrules.Rules{}.LiteralFor(f.view.ScopeOf(tt.subject), tt.give, tt.text, f.view)
				assert.False(t, read, "the text is no value of the type")
			})
		}
	})

	t.Run("Text", func(t *testing.T) {
		t.Parallel()

		str := &node.TypeRef{Spelling: strName}
		char := &node.TypeRef{Spelling: charName}
		abc := emit.Literal(emit.LiteralString, "abc")
		tests := []struct {
			name string
			give *node.TypeRef
			text string
			want emit.Value
		}{
			{name: "returns the string literal for str", give: str, text: "abc", want: abc},
			{
				name: "returns the string literal for a borrow of str",
				give: &node.TypeRef{Spelling: "&str", Form: symbol.FormBorrow, Elems: []*node.TypeRef{str}},
				text: "abc", want: abc,
			},
			{
				name: "returns the from of the literal for String",
				give: &node.TypeRef{Spelling: stringName},
				text: "abc",
				want: emit.Call(stringFrom, abc),
			},
			{
				name: "returns the new of the literal for Path",
				give: &node.TypeRef{Spelling: "Path", Package: "std/path"},
				text: "abc",
				want: emit.Call(pathNew, abc),
			},
			{
				name: "returns the from of the literal for a Box of str",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{str}}, text: "abc",
				want: emit.Call(boxFrom, abc),
			},
			{name: "returns the character literal for char", give: char, text: "a", want: emit.Raw(rust.Lang, "'a'")},
			{name: "returns the escape of a quote for char", give: char, text: "'", want: emit.Raw(rust.Lang, `'\''`)},
			{
				name: "returns the escape of a backslash for char",
				give: char,
				text: `\`,
				want: emit.Raw(rust.Lang, `'\\'`),
			},
			{
				name: "returns the escape of a line feed for char",
				give: char,
				text: "\n",
				want: emit.Raw(rust.Lang, `'\n'`),
			},
			{
				name: "returns the escape of a carriage return for char",
				give: char,
				text: "\r",
				want: emit.Raw(rust.Lang, `'\r'`),
			},
			{name: "returns the escape of a tab for char", give: char, text: "\t", want: emit.Raw(rust.Lang, `'\t'`)},
			{name: "returns the escape of nul for char", give: char, text: "\x00", want: emit.Raw(rust.Lang, `'\0'`)},
			{
				name: "returns the Unicode escape of a control character for char", give: char, text: "\x01",
				want: emit.Raw(rust.Lang, `'\u{1}'`),
			},
			{
				name: "returns the Unicode escape of delete for char", give: char, text: "\x7f",
				want: emit.Raw(rust.Lang, `'\u{7f}'`),
			},
			{
				name: "returns the string literal for char of two characters", give: char, text: "ab",
				want: emit.Literal(emit.LiteralString, "ab"),
			},
			{
				name: "returns the string literal for char of no character", give: char, text: "",
				want: emit.Literal(emit.LiteralString, ""),
			},
			{
				name: "returns the string literal for CString",
				give: &node.TypeRef{Spelling: "CString", Package: "std/ffi"},
				text: "abc",
				want: abc,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(
					t,
					rustrules.Rules{}.Text(tt.give, tt.text, rules.View{}),
					tt.want,
					"the value of the text",
				)
			})
		}

		t.Run("returns the value of a text that an author states for a String", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			sku := f.field(t, itemID, "sku")
			claim := meta.Claim{Subject: sku.ID, Authority: meta.AuthorityPlugin, Plugin: "fixture"}
			assert.NoError(t, meta.Stamp(f.Facts, f.Keys.Sample, "stated", claim), "the fixture stamp applies")
			sample, _ := rustrules.Rules{}.DeriveSamples(&node.TypeRef{Spelling: "Item", Target: itemID}, "", f.view)
			assert.True(t, sample.OK(), "the sample derives")
			assert.NotEmpty(t, sample.Value.Fields, "the literal has values")
			expect.Equal(t, sample.Value.Fields[0],
				emit.NamedField("sku", emit.Call(stringFrom, emit.Literal(emit.LiteralString, "stated"))),
				"the stated text takes String::from")
		})
	})
}

// TestValuesAllocs checks the ceilings of the values in the ordinary run,
// which does not run benchmarks.
func TestValuesAllocs(t *testing.T) {
	checkAllocs(t, valuesCalls())
}

// BenchmarkValues measures the pair of a number and of a String, the zero of
// a number, the literal of a String, and the text of a String and of a char
// under their ceilings.
func BenchmarkValues(b *testing.B) {
	benchCalls(b, valuesCalls())
}

// valuesCalls returns the calls of the values that the allocation test and
// the benchmark measure.
func valuesCalls() []allocCall {
	r := rustrules.Rules{}
	u8 := &node.TypeRef{Spelling: u8Name}
	owned := &node.TypeRef{Spelling: stringName}
	char := &node.TypeRef{Spelling: charName}
	var (
		sample rules.Sample
		zero   emit.Value
		value  emit.Value
	)
	return []allocCall{
		{
			name: "DeriveSamples", caseName: "a number", allocs: numberPairAllocs,
			call:  func() { sample, _ = r.DeriveSamples(u8, "n", rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, sample, rules.Of(n42u8), "DeriveSamples of u8") },
		},
		{
			name: "DeriveSamples", caseName: "a String", allocs: stringPairAllocs,
			call: func() { sample, _ = r.DeriveSamples(owned, "n", rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample, rules.Of(emit.Call(stringFrom, emit.Literal(emit.LiteralString, "n-a"))),
					"DeriveSamples of String")
			},
		},
		{
			name: "ZeroValue", allocs: zeroAllocs,
			call:  func() { zero, _ = r.ZeroValue(u8, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, zero, n0u8, "ZeroValue of u8") },
		},
		{
			name: "LiteralFor", allocs: stringLitAllocs,
			call: func() { value, _ = r.LiteralFor(rules.Scope{}, owned, `"a"`, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value, emit.Call(stringFrom, emit.Literal(emit.LiteralString, "a")),
					"LiteralFor of String")
			},
		},
		{
			name: "Text", caseName: "a String", allocs: stringTextAllocs,
			call: func() { value = r.Text(owned, "a", rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value, emit.Call(stringFrom, emit.Literal(emit.LiteralString, "a")), "Text of String")
			},
		},
		{
			name: "Text", caseName: "a char", allocs: charTextAllocs,
			call:  func() { value = r.Text(char, "a", rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, value, emit.Raw(rust.Lang, "'a'"), "Text of char") },
		},
	}
}
