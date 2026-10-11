// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"strconv"

	"go.dokimi.dev/eidos/core/symbol"
)

// StmtKind selects a statement's populated fields. The zero kind
// names no statement.
type StmtKind uint8

const (
	// StmtReturn returns Value, or returns bare when Value is
	// zero.
	StmtReturn StmtKind = iota + 1
	// StmtAssign binds Value to Names, one target or several,
	// because a delegate returning a result and a failure binds
	// two; Declare introduces the names.
	StmtAssign
	// StmtExpr evaluates Value for its effect: a delegate call.
	StmtExpr
	// StmtGuard runs Then when the value bound to Name reads as a
	// failure under the target language's convention. A language
	// whose failure propagates natively renders the guard as that
	// propagation, which may be nothing at all.
	StmtGuard
)

// String returns the kind's spelling. Faults and lint findings name
// statement kinds, so a consumer matching on the spelling matches
// on API. A kind nothing declares returns its number rather than a
// name.
func (k StmtKind) String() string {
	switch k {
	case StmtReturn:
		return "return"
	case StmtAssign:
		return "assign"
	case StmtExpr:
		return "expr"
	case StmtGuard:
		return "guard"
	default:
		return strconv.Itoa(int(k))
	}
}

// Stmt is one scaffolding statement: enough to delegate, too
// little to write logic in. Kind selects the populated fields, so
// a consumer switches on it and reads without an assertion. A Stmt
// is a plain value: copy it freely.
type Stmt struct {
	Kind    StmtKind `json:"kind"`
	Declare bool     `json:"declare,omitzero"`
	// Name is the guarded name; Names are the assignment targets.
	Name  string   `json:"name,omitzero"`
	Names []string `json:"names,omitzero"`
	Value Expr     `json:"value,omitzero"`
	Then  []Stmt   `json:"then,omitzero"`
}

// ExprKind selects an expression's populated fields. The zero kind
// names no expression, which is what a bare return's Value holds.
type ExprKind uint8

const (
	// ExprName is a local spelling: a parameter, a receiver, a
	// package-local callable, a dotted path onto one. Nothing
	// resolves it and nothing imports for it. A declaration of
	// another package is an [ExprRef].
	ExprName ExprKind = iota + 1
	// ExprCall applies Fn to Args.
	ExprCall
	// ExprValue carries a Value: a sample, an alternate or a zero
	// the projection derived, spelled by the target's own scaffold
	// and qualified for the file it is written into. A value can
	// refer to types and callees that the writing file does not
	// import yet, so the target resolves them.
	ExprValue
	// ExprRef refers to a function, a constant or a variable that
	// another package declares, by its identity in Ref. The target
	// qualifies the name for the file that it is written into and
	// records the import, as it does for the callee of a call value.
	// As the Fn of a call, it calls the function with the call's
	// arguments, which can be local names.
	ExprRef
)

// String returns the kind's spelling, and the number for a kind
// nothing declares. The spellings are API for the same reason the
// statement kinds' are.
func (k ExprKind) String() string {
	switch k {
	case ExprName:
		return "name"
	case ExprCall:
		return "call"
	case ExprValue:
		return "value"
	case ExprRef:
		return "ref"
	default:
		return strconv.Itoa(int(k))
	}
}

// Expr is one scaffolding expression. The zero Expr names nothing,
// which is what a bare return carries. A copy of an Expr shares
// what Fn, Val and Args reach. Fn is a pointer because a struct
// cannot contain itself.
type Expr struct {
	Kind ExprKind `json:"kind"`
	Name string   `json:"name,omitzero"`
	Fn   *Expr    `json:"fn,omitzero"`
	Args []Expr   `json:"args,omitzero"`
	// Val is the carried value, populated on ExprValue alone. It is
	// a pointer because the zero expression carries none, and a
	// value tree is larger than the rest of the expression.
	Val *Value `json:"val,omitzero"`
	// Ref identifies the declaration of an ExprRef, populated on
	// ExprRef alone. It is a pointer for the reason Val is one, and so
	// that an encoder omits it without a call of Identity.IsZero.
	Ref *symbol.Identity `json:"ref,omitzero"`
}

// ValueExpr returns an expression carrying one derived value.
func ValueExpr(v Value) Expr { return Expr{Kind: ExprValue, Val: &v} }

// RefExpr returns an expression that refers to the declaration of
// another package with the identity id. It allocates the identity that
// it points at.
func RefExpr(id symbol.Identity) Expr { return Expr{Kind: ExprRef, Ref: &id} }
