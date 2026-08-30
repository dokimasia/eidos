// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit

import "strconv"

// StmtKind selects a statement's populated fields. The zero kind
// names no statement.
type StmtKind uint8

const (
	// StmtReturn returns Value, or returns bare when Value is
	// zero.
	StmtReturn StmtKind = iota + 1
	// StmtAssign binds Value to Names, one target or several,
	// because a delegate answering a result and a failure binds
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

// String answers the kind's spelling. Faults and lint findings name
// statement kinds, so a consumer matching on the spelling matches
// on API. A kind nothing declares answers its number rather than a
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
	Kind StmtKind `json:"kind"`
	// Name is the guarded name; Names are the assignment targets.
	Name    string   `json:"name,omitzero"`
	Names   []string `json:"names,omitzero"`
	Value   Expr     `json:"value,omitzero"`
	Declare bool     `json:"declare,omitzero"`
	Then    []Stmt   `json:"then,omitzero"`
}

// ExprKind selects an expression's populated fields. The zero kind
// names no expression, which is what a bare return's Value holds.
type ExprKind uint8

const (
	// ExprName is a local spelling: a parameter, a receiver, a
	// package-local callable, a dotted path onto one. Nothing
	// resolves it and nothing imports for it; a name that needs
	// qualification is a type spelling, which belongs to the
	// lowering seam rather than to scaffolding.
	ExprName ExprKind = iota + 1
	// ExprCall applies Fn to Args.
	ExprCall
)

// String answers the kind's spelling, and the number for a kind
// nothing declares. The spellings are API for the same reason the
// statement kinds' are.
func (k ExprKind) String() string {
	switch k {
	case ExprName:
		return "name"
	case ExprCall:
		return "call"
	default:
		return strconv.Itoa(int(k))
	}
}

// Expr is one scaffolding expression. The zero Expr names nothing,
// which is what a bare return carries. Fn is the one pointer in
// the vocabulary, because a struct cannot hold itself.
type Expr struct {
	Kind ExprKind `json:"kind"`
	Name string   `json:"name,omitzero"`
	Fn   *Expr    `json:"fn,omitzero"`
	Args []Expr   `json:"args,omitzero"`
}
