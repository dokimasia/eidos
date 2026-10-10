// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the fixture's modules.
const (
	aFile      = "fx/a.ts"
	aPkg       = "fx/a"
	depPkg     = "fx/dep/d"
	barrelPkg  = "fx/barrel"
	morePkg    = "fx/more"
	starPkg    = "fx/star"
	innerPkg   = "fx/a/N"
	outsideLib = "outside-lib"
)

// The names of the fixture's declarations that the cases read.
const (
	rowName       = "Row"
	targetName    = "Target"
	derivedName   = "Derived"
	readerName    = "Reader"
	signalName    = "Signal"
	loadName      = "load"
	rowsName      = "rows"
	abortSignal   = "AbortSignal"
	numberName    = "number"
	stringName    = "string"
	wordCheck     = "check"
	derivedPascal = "RowCheck"
)

// rulesAllocs is the allocation count of one projection pass over the
// fixture tree, which [rulestest.BenchRules] measures. The pass allocated
// 462 times at 2,000 iterations, and 461 or 462 times in each of 40
// fresh runs of one iteration. A memory profile attributes 189
// allocations to TypeOf, 150 to SamplesOf, 78 to MembersOf and 23 to
// CallableOf. The profile samples the tiny allocations of the rest only
// in part.
const rulesAllocs = 462

// The allocations of a signature's classification and a derived name.
const (
	// returnRolesAllocs counts the list of the roles of the returns.
	returnRolesAllocs = 1
	// typeNameAllocs counts the word in Pascal case and the joined name.
	typeNameAllocs = 2
)

// The identities of the fixture's declarations that more than one case
// compares.
var (
	rowID     = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: rowName, Kind: symbol.KindStruct}
	targetID  = symbol.Identity{Lang: typescript.Lang, Package: depPkg, Name: targetName, Kind: symbol.KindStruct}
	derivedID = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: derivedName, Kind: symbol.KindStruct}
)

// fixtureTree is the TypeScript source that the cases project. It has a
// module that exercises every rule, the module that it imports a class
// from, a barrel that re-exports, the modules behind the barrel, and a
// script that declares into the global scope.
var fixtureTree = fstest.MapFS{
	aFile: {Data: []byte(`import { Target } from "./dep/d";
import * as dep from "./dep/d";
import Main, { Again as Renamed, Starred, Shown } from "./barrel";
import Relayed from "./relay";
import { Ghost } from "./cycle1";
import { Lost } from "./lost";
import { Outside } from "outside-lib";
import * as lib from "outside-lib";

/** Row has a field of every type in the builtin table. */
export class Row {
  id!: number;
  name!: string;
  tags!: string[];
  next?: Row;
  when!: Date;
  bytes!: Uint8Array;
  big!: bigint;
  target!: Target;
  counts!: Record<string, number>;
  ok!: boolean;
  pair!: [string, number];
  mode!: "read" | "write";
  list!: Array<number>;
  map!: Map<string, number>;
  main!: Main;
  renamed!: Renamed;
  starred!: Starred;
  outside!: Outside;
  qualified!: dep.Target;
  private hidden!: number;
  static total = 0;
}

/** Bare has nothing that an object literal can set. */
export class Bare {
  private hidden!: number;
}

export class Base {
  kind!: string;
  protected secret!: string;
  private own!: string;
  constructor(kind: string) {}
  protected guard(): void {}
}

export class Derived extends Base {
  label!: string;
  helper(): void {}
}

export class Accessors {
  get size(): number {
    return 0;
  }
  set size(v: number) {}
  get readOnly(): string {
    return "";
  }
  set writeOnly(v: string) {}
  static get shared(): number {
    return 0;
  }
}

export class Overloaded {
  constructor(a: string);
  constructor(a: number);
  constructor(a: any) {}
}

export class Hidden {
  private constructor() {}
}

export class Generic<T> {
  item!: T;
  constructor(item: T) {}
}

export class Concrete extends Generic<string> {}

export class Bounded<T extends Row> {
  item!: T;
}

export class L0 extends L1 {}
export class L1 extends L2 {}
export class L2 extends L3 {}
export class L3 extends L4 {}
export class L4 extends L5 {}
export class L5 extends L6 {}
export class L6 extends L7 {}
export class L7 extends L8 {}
export class L8 {}

export class OffReader extends Optional {}

export interface Reader {
  read(p: number): Promise<number>;
  readonly size: number;
  label?: string;
}

export interface Optional {
  label?: string;
}

export interface Closer extends Reader {
  close(signal: AbortSignal): void;
}

export enum Color {
  Red,
  Green,
  Blue,
}

export enum Mode {
  Read = "read",
  Write = "write",
}

export enum Flags {
  None = 0,
  A = 1 << 0,
  B = 1 << 1,
  AB = A | B,
}

export enum Same {
  One = 1,
  Uno = 1,
}

export enum Codes {
  Ok = 200,
  NotFound = 404,
}

export type Plain = number;
export type Signal = AbortSignal;
export type Box<T> = { item: T };
export type Pair<K, V> = [K, V];
export type Deep = Deep[];
export type Loop1 = Loop2;
export type Loop2 = Loop1;

/** Shapes has a field of every form whose values the rules derive or refuse. */
export class Shapes {
  lookup!: { [key: string]: number };
  either!: string | number;
  color!: Color;
  plain!: Plain;
  box!: Box<string>;
  couple!: Pair<string, number>;
  byColor!: Record<Color, number>;
  opt!: Optional;
  bare!: Bare;
  fn!: () => void;
  both!: Reader & Optional;
  deep!: Deep;
  anything!: unknown;
  only!: "read";
  empty!: [];
  broken!: [string, Map<string, number>];
  same!: Same;
  maybe!: string | undefined;
  nullable!: string | null;
  absent!: string | null | undefined;
  stream!: AsyncIterable<number>;
  computed!: Computed;
  mixed!: Mixed;
  mode!: Mode;
}

export enum Computed {
  X = "x".length,
  Y = 1,
  Z = 2,
}

export class Mixed {
  map!: Map<string, number>;
  id!: number;
}

export class Repo {
  get(id: number): Row {
    return undefined as any;
  }
}

export function untyped(a) {
  return a;
}

declare global {
  interface Shared {
    s: number;
  }
}

export function load(signal: AbortSignal, id: number): Promise<Row> {
  return undefined as any;
}

export async function* rows(): AsyncIterable<Row> {}

export function find(id: number): Row | undefined {
  return undefined;
}

export function touch(t: Target): void {}

export const limit = 16;
export let current: Row | null = null;

export namespace N {
  export interface Inner {
    v: number;
  }
}
`)},
	"fx/dep/d.ts": {Data: []byte(`/** Target is the class that the module a imports. */
export class Target {
  v!: number;
}
`)},
	"fx/barrel.ts": {Data: []byte(`export { Again } from "./more";
export * from "./star";
export default class Main {}
class Hidden2 {}
export { Hidden2 as Shown };
`)},
	"fx/relay.ts": {Data: []byte(`import { Again } from "./more";
export default Again;
`)},
	"fx/cycle1.ts": {Data: []byte(`export * from "./cycle2";
`)},
	"fx/cycle2.ts": {Data: []byte(`export * from "./cycle1";
`)},
	"fx/more.ts": {Data: []byte(`export class Again {}
`)},
	"fx/star.ts": {Data: []byte(`export interface Starred {}
`)},
	"fx/globals.ts": {Data: []byte(`interface GlobalThing {
  g: number;
}
`)},
}

// allocCall is one measured call that an allocation test and a benchmark
// share.
type allocCall struct {
	name     string // the measured method, which is also the benchmark's name
	caseName string // the case of a method with more than one call
	allocs   uint64 // the allocation ceiling
	call     func()
	check    func(tb assert.TB) // checks the result that the call leaves
	// bench measures a case whose ceiling only a benchmark checks, in
	// place of call. No list that an allocation test reads contains such
	// a case.
	bench func(b *testing.B)
}

// fixture is the loaded tree with a tracked view over it.
type fixture struct {
	*rulestest.Fixture
	view  rules.View
	reads *store.ReadSet
}

// TestRules checks each classification that the TypeScript rules make
// for the kernel.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rules that meet the projection contract over the fixture tree", func(t *testing.T) {
			t.Parallel()

			rulestest.RunRulesSuite(t, setup)
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns TypeScript", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tsrules.New().Lang(), typescript.Lang, "Lang returns TypeScript")
		})
	})

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a policy that merges the extends clause and then the implements clause", func(t *testing.T) {
			t.Parallel()

			policy := tsrules.New().Members()
			expect.Equal(t, policy.Contributes,
				[]rules.Contribution{rules.ContributesExtends, rules.ContributesImplements},
				"the policy walks the extends clause and then the implements clause")
			expect.Equal(t, policy.Shadowing, rules.ShadowMerge, "the policy keeps every arrival as an overload")
			expect.Equal(t, policy.Depth, 0, "the policy walks to the kernel's default depth")
			expect.False(t, policy.EmbedsAreFields, "TypeScript has no embedded fields")
		})

		t.Run("returns a policy under which a class has the members of the class it extends", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			derived := f.decl(t, aPkg, derivedName, symbol.KindStruct)
			set, is := rules.NewBound(tsrules.New(), f.view, nil).MembersOf(derived)
			assert.True(t, is, "the walk accepts a class")
			var names []string
			for _, m := range set.Members {
				if decl, named := m.Symbol.(node.Declaration); named {
					names = append(names, decl.Identity().Name)
				}
			}
			assert.Equal(t, names, []string{"label", "helper", "kind", "secret", "own", "constructor", "guard"},
				"the members of Derived come first, and the members of Base follow")
		})
	})

	t.Run("ParamRole", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give *node.TypeRef
			want rules.ParamRole
		}{
			{
				name: "classifies an AbortSignal parameter as the context",
				give: &node.TypeRef{Spelling: abortSignal},
				want: rules.ParamContext,
			},
			{
				name: "classifies an optional AbortSignal parameter as the context",
				give: &node.TypeRef{
					Spelling: abortSignal + "|undefined",
					Form:     symbol.FormOptional,
					Elems:    []*node.TypeRef{{Spelling: abortSignal}},
				},
				want: rules.ParamContext,
			},
			{
				name: "classifies an AbortSignal that an import binds as an input",
				give: &node.TypeRef{Spelling: abortSignal, Package: outsideLib}, want: rules.ParamInput,
			},
			{
				name: "classifies a parameter of any other type as an input",
				give: &node.TypeRef{Spelling: numberName},
				want: rules.ParamInput,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tsrules.New().ParamRole(&node.Param{Type: tt.give}, rules.View{}), tt.want,
					"ParamRole returns the role of the parameter")
			})
		}

		t.Run("classifies a parameter without a type as an input", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, tsrules.New().ParamRole(&node.Param{}, rules.View{}), rules.ParamInput,
				"a parameter without a type is an input")
			expect.Equal(t, tsrules.New().ParamRole(nil, rules.View{}), rules.ParamInput,
				"a nil parameter is an input")
		})

		t.Run("classifies a parameter of an alias of AbortSignal as the context", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			target := symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: signalName, Kind: symbol.KindAlias}
			signal := &node.TypeRef{Spelling: signalName, Target: target}
			assert.Equal(t, tsrules.New().ParamRole(&node.Param{Type: signal}, f.view), rules.ParamContext,
				"Signal is an alias of AbortSignal")
		})

		aliases := []struct {
			name string
			give string
		}{
			{name: "classifies a parameter of an alias that the view does not contain as an input", give: "Missing"},
			{name: "classifies a parameter of a cycle of aliases as an input", give: "Loop1"},
		}
		for _, tt := range aliases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				target := symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: tt.give, Kind: symbol.KindAlias}
				ref := &node.TypeRef{Spelling: tt.give, Target: target}
				assert.Equal(t, tsrules.New().ParamRole(&node.Param{Type: ref}, f.view), rules.ParamInput,
					"the walk over the aliases does not end at AbortSignal")
			})
		}

		t.Run("classifies the AbortSignal of a loaded function as the context", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			c, _ := rules.NewBound(tsrules.New(), f.view, nil).CallableOf(f.function(t, loadName))
			expect.Equal(t, c.Params[0].Role, rules.ParamContext, "the signal is the context")
			expect.Equal(t, c.Params[1].Role, rules.ParamInput, "the id is an input")
		})
	})

	t.Run("ReturnRoles", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies an asynchronous stream as a stream", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			c, _ := rules.NewBound(tsrules.New(), f.view, nil).CallableOf(f.function(t, rowsName))
			assert.Equal(t, c.Returns[0].Role, rules.ReturnStream, "rows returns an AsyncIterable")
		})

		streams := []string{"AsyncIterable", "AsyncIterableIterator", "AsyncGenerator"}
		for _, spelling := range streams {
			t.Run("classifies "+spelling+" as a stream", func(t *testing.T) {
				t.Parallel()

				ret := &node.Return{
					Type: &node.TypeRef{Spelling: spelling, Args: []*node.TypeRef{{Spelling: numberName}}},
				}
				roles, _ := tsrules.New().ReturnRoles([]*node.Return{ret}, rules.View{})
				assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnStream},
					"ReturnRoles classifies an asynchronous stream of numbers as a stream")
			})
		}

		t.Run("classifies every other return as a value", func(t *testing.T) {
			t.Parallel()

			rets := []*node.Return{{Type: &node.TypeRef{Spelling: numberName}}, nil, {}}
			roles, _ := tsrules.New().ReturnRoles(rets, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue, rules.ReturnValue},
				"a number, a nil return and a return without a type are values")
		})

		t.Run("reports the raised error model for every callable", func(t *testing.T) {
			t.Parallel()

			_, model := tsrules.New().ReturnRoles(nil, rules.View{})
			assert.Equal(t, model, rules.ErrorsRaised, "a TypeScript callable throws without declaring it")
		})
	})

	t.Run("TypeName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			word string
			base string
			want string
		}{
			{name: "joins the word in Pascal case onto the base", word: wordCheck, base: rowName, want: derivedPascal},
			{name: "keeps a base as the author wrote it", word: wordCheck, base: "httpRow", want: "httpRowCheck"},
			{name: "keeps a word in Pascal case", word: "Mock", base: rowName, want: "RowMock"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tsrules.New().TypeName(tt.word, tt.base), tt.want,
					"TypeName returns the joined name")
			})
		}
	})
}

// TestRulesAllocs checks that the classifications allocate only the list
// of roles, and that a derived name allocates only the name. The ordinary
// test run runs no benchmark, so this test checks the ceilings.
func TestRulesAllocs(t *testing.T) {
	checkAllocs(t, rulesCalls())
}

// BenchmarkRules measures each classification of the TypeScript rules,
// and runs the four hot projections over the fixture tree under their
// ceiling. Only -bench checks the ceiling of the projections, because
// the pass builds the fixture's view.
func BenchmarkRules(b *testing.B) {
	benchCalls(b, append(rulesCalls(), allocCall{
		name: "New", caseName: "the projections of the fixture tree",
		bench: func(b *testing.B) {
			b.Helper()
			rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: rulesAllocs})
		},
	}))
}

// rulesCalls returns a call of the constructor and of every
// classification of rules.go.
func rulesCalls() []allocCall {
	r := tsrules.New()
	signal := &node.Param{Type: &node.TypeRef{Spelling: abortSignal}}
	rets := []*node.Return{{Type: &node.TypeRef{Spelling: numberName}}}
	var (
		built  rules.SourceRules
		lang   symbol.Lang
		policy rules.MemberPolicy
		role   rules.ParamRole
		roles  []rules.ReturnRole
		name   string
	)
	return []allocCall{
		{
			name:     "New",
			caseName: "the rules",
			call:     func() { built = tsrules.New() },
			check:    func(tb assert.TB) { assert.Equal(tb, built.Lang(), typescript.Lang, "New returns the rules") },
		},
		{
			name:  "Lang",
			call:  func() { lang = r.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, typescript.Lang, "Lang returns TypeScript") },
		},
		{
			name: "Members",
			call: func() { policy = r.Members() },
			check: func(tb assert.TB) {
				assert.Equal(tb, policy.Shadowing, rules.ShadowMerge, "Members returns the merging policy")
			},
		},
		{
			name: "ParamRole",
			call: func() { role = r.ParamRole(signal, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, role, rules.ParamContext, "ParamRole classifies the context")
			},
		},
		{
			name:   "ReturnRoles",
			allocs: returnRolesAllocs,
			call:   func() { roles, _ = r.ReturnRoles(rets, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, roles, []rules.ReturnRole{rules.ReturnValue}, "ReturnRoles classifies the value")
			},
		},
		{
			name:   "TypeName",
			allocs: typeNameAllocs,
			call:   func() { name = r.TypeName(wordCheck, rowName) },
			check: func(tb assert.TB) {
				assert.Equal(tb, name, derivedPascal, "TypeName joins the word onto the base")
			},
		},
	}
}

// checkAllocs checks the ceiling of every call in the ordinary test run,
// and the result that each call leaves.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its
// ceiling. It runs one sub-benchmark for each method, in the order of
// first appearance, and one sub-benchmark inside it for each case of a
// method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if _, seen := byMethod[c.name]; !seen {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling,
// and checks the result that the last call leaves. The call runs once
// before the contract starts, so the count leaves out what the first
// call initialises. A case that sets bench runs bench instead.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	if tt.bench != nil {
		tt.bench(b)
		return
	}
	tt.call()
	c := bench.Start(b).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// loaded returns the fixture with a tracked view over it.
func loaded(tb assert.TB) *fixture {
	tb.Helper()

	_, f := setup(tb)
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	return &fixture{
		Fixture: f, reads: reads,
		view: rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys},
	}
}

// decl looks up a top-level declaration of the fixture by its package,
// its name and its kind.
func (f *fixture) decl(tb assert.TB, pkg, name string, kind symbol.Kind) symbol.Symbol {
	tb.Helper()

	want := symbol.Identity{Lang: typescript.Lang, Package: pkg, Name: name, Kind: kind}
	sym, held := f.Graph.Lookup(want)
	assert.True(tb, held, "the fixture declares "+want.String())
	return sym
}

// function returns a function of the fixture's module a by its name.
func (f *fixture) function(tb assert.TB, name string) *node.Function {
	tb.Helper()

	pkg, held := f.Graph.PackageOf(symbol.Identity{Lang: typescript.Lang, Package: aPkg, Kind: symbol.KindPackage})
	assert.True(tb, held, "the module a loaded")
	var found *node.Function
	for decl := range node.Declarations(pkg) {
		if fn, is := decl.(*node.Function); is && fn.Name == name && found == nil {
			found = fn
		}
	}
	assert.NotNil(tb, found, "the module a declares the function "+name)
	return found
}

// class returns a class of the fixture's module a.
func (f *fixture) class(tb assert.TB, name string) *node.Struct {
	tb.Helper()

	class, is := f.decl(tb, aPkg, name, symbol.KindStruct).(*node.Struct)
	assert.True(tb, is, name+" is a class")
	return class
}

// field returns a field of a class of the fixture's module a.
func (f *fixture) field(tb assert.TB, host, name string) *node.Field {
	tb.Helper()

	var found *node.Field
	for _, candidate := range f.class(tb, host).Fields {
		if candidate.Name == name && found == nil {
			found = candidate
		}
	}
	assert.NotNil(tb, found, host+" declares the field "+name)
	return found
}

// file returns the File node that a file of the fixture contributes to a
// package.
func (f *fixture) file(tb assert.TB, pkg, path string) *node.File {
	tb.Helper()

	p, held := f.Graph.PackageOf(symbol.Identity{Lang: typescript.Lang, Package: pkg, Kind: symbol.KindPackage})
	assert.True(tb, held, "the package "+pkg+" loaded")
	var found *node.File
	for _, file := range p.Files {
		if file.Path == path && found == nil {
			found = file
		}
	}
	assert.NotNil(tb, found, "the file "+path+" contributes to the package "+pkg)
	return found
}

// setup returns the TypeScript rules and the loaded fixture tree for the
// suite.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()

	return tsrules.New(), rulestest.Loaded(tb, tsfrontend.New(), fixtureTree)
}
