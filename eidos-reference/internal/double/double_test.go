// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package double_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/double"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The tree of the cases, its Go package and its TypeScript module, and the
// interfaces that the tree declares.
const (
	tree              = "testdata/tree"
	goPackage         = "example.com/doubles"
	constraintPackage = "example.com/doubles/constraint"
	tsPackage         = "shaped"
	ledgerName        = "Ledger"
	counterName       = "Counter"
	storeName         = "Store"
	streamName        = "Stream"
	shapedName        = "Shaped"
	numberName        = "Number"
	doubleSuffix      = "Double"
)

// probeID is the name of the plugin that mirrors the interfaces.
const probeID plugin.ID = "probe"

// The names that the doubles of Ledger, Counter and Store give the receivers
// and a parameter without a name. positionalName is the name of the
// position of the second parameter, and clashParam is the name that an
// unnamed second parameter takes where another name of its method is
// positionalName.
const (
	receiverName   = "l"
	clashReceiver  = "l0"
	resultReceiver = "c0"
	unnamedParam   = "a0"
	positionalName = "a1"
	clashParam     = "a10"
	fallback       = "d"
	getName        = "Get"
	putName        = "Put"
	closeName      = "Close"
	hostWithMark   = "_Double"
	intType        = "int"
	errorType      = "error"
	stateName      = "state"
	// appendName and lenName are predeclared functions of Go.
	appendName = "append"
	lenName    = "len"
)

// mirrored is what Of returned for one interface.
type mirrored struct {
	methods []*emit.Method
	ok      bool
}

// Of and Copy return the methods of a double, each with a receiver and a
// name for each parameter. Signatures returns the signatures of methods
// without a receiver.
func TestDouble(t *testing.T) {
	t.Parallel()

	t.Run("Of", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a method for each method of the interface", func(t *testing.T) {
			t.Parallel()

			got, _ := of(t, goPackage)
			ledger := got[ledgerName]
			assert.True(t, ledger.ok, "Ledger has a double")
			assert.Length(t, ledger.methods, 2, "the double of Ledger has its two methods")
			expect.That(t, ledger.methods[0].Name).Equal(getName, "the first method is Get")
			expect.That(t, ledger.methods[1].Name).Equal(putName, "the second method is Put")
		})

		t.Run("names a parameter without a name by its position", func(t *testing.T) {
			t.Parallel()

			got, _ := of(t, goPackage)
			assert.Equal(t, got[ledgerName].methods[0].Params.Items()[0].Name, unnamedParam,
				"the parameter of Get takes the name of its position")
		})

		t.Run("names a parameter without a name apart from a named result", func(t *testing.T) {
			t.Parallel()

			got, _ := of(t, goPackage)
			assert.Equal(t, got[storeName].methods[0].Params.Items()[1].Name, clashParam,
				"the second parameter of Put takes a number, because the result of Put is a1")
		})

		t.Run("names the receiver by the first letter of the double", func(t *testing.T) {
			t.Parallel()

			got, _ := of(t, goPackage)
			assert.Equal(t, got[ledgerName].methods[0].Receiver.Name, receiverName,
				"the receiver of Get takes the first letter of LedgerDouble")
		})

		t.Run("names the receiver apart from a parameter with the first letter", func(t *testing.T) {
			t.Parallel()

			got, _ := of(t, goPackage)
			assert.Equal(t, got[ledgerName].methods[1].Receiver.Name, clashReceiver,
				"the receiver of Put takes a number, because a parameter of Put is l")
		})

		t.Run("names the receiver apart from a result with the first letter", func(t *testing.T) {
			t.Parallel()

			got, _ := of(t, goPackage)
			assert.Equal(t, got[counterName].methods[0].Receiver.Name, resultReceiver,
				"the receiver of Count takes a number, because the result of Count is c")
		})

		t.Run("reports IncompleteMembers at an interface whose embed does not resolve", func(t *testing.T) {
			t.Parallel()

			got, sink := of(t, goPackage)
			assert.False(t, got[streamName].ok, "Stream has no double")
			assert.Equal(t, codes(sink), []diag.Code{double.IncompleteMembers}, "Of reports the gap of Stream")
		})

		t.Run("reports Property at an interface with a property", func(t *testing.T) {
			t.Parallel()

			got, sink := of(t, tsPackage)
			assert.False(t, got[shapedName].ok, "Shaped has no double")
			assert.Equal(t, codes(sink), []diag.Code{double.Property}, "Of reports the property of Shaped")
		})

		t.Run("reports Constraint at an interface with type-set elements", func(t *testing.T) {
			t.Parallel()

			got, sink := of(t, constraintPackage)
			assert.False(t, got[numberName].ok, "Number has no double")
			assert.Equal(t, codes(sink), []diag.Code{double.Constraint}, "Of reports the type set of Number")
		})
	})

	t.Run("Copy", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a copy of each method with lists of its own", func(t *testing.T) {
			t.Parallel()

			i := emitted()
			got := double.Copy(ledgerName+doubleSuffix, i)
			assert.Length(t, got, 1, "the copy has the one method of the interface")
			source := i.Methods.Items()[0]
			expect.That(t, got[0].Name).Equal(closeName, "the copy has the name of the method")
			expect.NotEqual(t, got[0].Params.Items()[0], source.Params.Items()[0],
				"the copy has parameters of its own", assert.ByIdentity())
			expect.NotEqual(t, got[0].Returns.Items()[0], source.Returns.Items()[0],
				"the copy has results of its own", assert.ByIdentity())
			expect.Equal(t, got[0].Returns.Items()[0].Type, source.Returns.Items()[0].Type,
				"the copy shares the type of a result", assert.ByIdentity())
		})

		t.Run("names the receiver d for a double whose name does not start with a letter", func(t *testing.T) {
			t.Parallel()

			got := double.Copy(hostWithMark, emitted())
			assert.Equal(t, got[0].Receiver.Name, fallback, "the receiver takes the fallback letter")
		})

		t.Run("names a parameter without a name apart from a named parameter", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: storeName}
			i.Methods.Append(&emit.Method{
				Name: putName,
				Params: emit.SlotOf(
					&emit.Param{Name: positionalName, Type: &emit.TypeRef{Spelling: intType}},
					&emit.Param{Type: &emit.TypeRef{Spelling: intType}},
				),
			})
			got := double.Copy(storeName+doubleSuffix, i)
			assert.Equal(t, got[0].Params.Items()[1].Name, clashParam,
				"the second parameter of Put takes a number, because the first parameter of Put is a1")
		})

		t.Run("renames a parameter and a result with the name of a predeclared identifier of Go", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: storeName}
			i.Methods.Append(&emit.Method{
				Name: putName,
				Params: emit.SlotOf(
					&emit.Param{Name: appendName, Type: &emit.TypeRef{Spelling: intType}},
					&emit.Param{Name: appendName + "0", Type: &emit.TypeRef{Spelling: intType}},
				),
				Returns: emit.SlotOf(&emit.Return{Name: lenName, Type: &emit.TypeRef{Spelling: intType}}),
			})
			got := double.Copy(storeName+doubleSuffix, i)
			params, returns := got[0].Params.Items(), got[0].Returns.Items()
			expect.That(t, params[0].Name).Equal(appendName+"1",
				"append takes the smallest number that the method does not have")
			expect.That(t, params[1].Name).Equal(appendName+"0", "a name that Go does not predeclare stays")
			expect.That(t, returns[0].Name).Equal(lenName+"0", "a named result takes a number too")
		})
	})

	t.Run("Signatures", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a copy of each method without its receiver", func(t *testing.T) {
			t.Parallel()

			got := double.Signatures(double.Copy(ledgerName+doubleSuffix, emitted()))
			assert.Length(t, got, 1, "the copy has the one method of the double")
			expect.That(t, got[0].Name).Equal(closeName, "the copy has the name of the method")
			expect.Nil(t, got[0].Receiver, "the copy has no receiver")
		})

		t.Run("returns a copy without the statements of the body", func(t *testing.T) {
			t.Parallel()

			source := emitted().Methods.Items()[0]
			source.Body.Prologue.Append(
				emit.Stmt{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: stateName}},
			)
			got := double.Signatures([]*emit.Method{source})
			assert.Length(t, got, 1, "the copy has the one method")
			assert.Empty(t, got[0].Body.Prologue.Items(), "the prologue of the copy is empty")
		})
	})
}

// of mirrors every interface of the package pkg of the tree through Of,
// in a generator of its own, and returns what Of returned for each
// interface by name, and the findings of the run.
func of(t *testing.T, pkg string) (map[string]mirrored, *diag.Sink) {
	t.Helper()

	got := map[string]mirrored{}
	p := sdk.NewPlugin(probeID).
		Handle(sdk.OnInterface(func(m *sdk.InterfaceMatch, _ *sdk.Emitter) error {
			methods, ok := double.Of(m, m.Interface.Name+doubleSuffix, m.Interface)
			got[m.Interface.Name] = mirrored{methods: methods, ok: ok}
			return nil
		})).
		Generator()
	f := referencetest.New(t, referencetest.Spec{
		Dir:      tree,
		Packages: []string{pkg},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	return got, res.Sink
}

// emitted returns an interface that a plugin emitted, with the method
// Close of one parameter and one result.
func emitted() *emit.Interface {
	i := &emit.Interface{Name: ledgerName}
	i.Methods.Append(&emit.Method{
		Name:    closeName,
		Params:  emit.SlotOf(&emit.Param{Name: receiverName, Type: &emit.TypeRef{Spelling: intType}}),
		Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: errorType}}),
	})
	return i
}

// codes returns the codes of the findings of the sink, in order.
func codes(sink *diag.Sink) []diag.Code {
	var out []diag.Code
	for d := range sink.All() {
		out = append(out, d.Code)
	}
	return out
}
