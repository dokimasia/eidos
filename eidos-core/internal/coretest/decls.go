// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The names the every-kind fixture declares, one per kind. A case
// asserting over the whole vocabulary reads the kind out of the
// name, so a failure message says which kind failed without a
// lookup.
const (
	FunctionName    = "Load"
	ParamName       = "input"
	ReturnName      = "output"
	MethodName      = "Scan"
	EnumName        = "Status"
	EnumVariantName = "StatusOpen"
	SumName         = "Result"
	SumVariantName  = "ResultOk"
	FieldName       = "Column"
	VariableName    = "Registry"
	ConstantName    = "Version"
	InterfaceName   = "Reader"
	AliasName       = "RowID"

	// ForeignName is what [Foreign] is called, which no fixture of a
	// model kind answers to.
	ForeignName = "Ghost"
)

// Function returns a function declaration in one package, carrying
// the identity the resolution step would have assigned it.
func Function(path, name string) *node.Function {
	return &node.Function{
		ID:         ID(path, name, symbol.KindFunction),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Params:     []*node.Param{Param(path, name, ParamName)},
		Returns:    []*node.Return{Return(path, name, ReturnName)},
	}
}

// Param returns a named parameter of one callable, carrying the
// identity the resolution step assigns it under the callable's
// name. The fixture's parameters spell no type, so every
// discriminator is empty, the way the step spells it.
func Param(path, callable, name string) *node.Param {
	return &node.Param{ID: MemberID(path, callable, name, symbol.KindParam), Name: name}
}

// Return returns a named result of one callable, carrying the
// identity the resolution step assigns it under the callable's
// name.
func Return(path, callable, name string) *node.Return {
	return &node.Return{ID: MemberID(path, callable, name, symbol.KindReturn), Name: name}
}

// Method returns a method attached to host, carrying the identity
// the resolution step assigns it under host's name, and host's
// identity in its back-pointer the way a loaded member does.
func Method(path, host, name string) *node.Method {
	return &node.Method{
		ID:         MemberID(path, host, name, symbol.KindMethod),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Host:       ID(path, host, symbol.KindStruct),
	}
}

// Field returns a field on host, carrying the identity the
// resolution step assigns it under host's name, and host's identity
// in its back-pointer the way a loaded member does.
func Field(path, host, name string) *node.Field {
	return &node.Field{
		ID:         MemberID(path, host, name, symbol.KindField),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Mutability: symbol.MutabilityMutable,
		Host:       ID(path, host, symbol.KindStruct),
	}
}

// Enum returns an enumeration holding one variant per name, each
// under the enum's name and carrying the enum's identity in its
// back-pointer.
func Enum(path, name string, variants ...string) *node.Enum {
	held := make([]*node.EnumVariant, 0, len(variants))
	for _, variant := range variants {
		held = append(held, &node.EnumVariant{
			ID:   MemberID(path, name, variant, symbol.KindEnumVariant),
			Name: variant,
			Host: ID(path, name, symbol.KindEnum),
		})
	}
	return &node.Enum{
		ID:         ID(path, name, symbol.KindEnum),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Variants:   held,
	}
}

// Sum returns a sum type holding one variant per name, each under
// the sum's name and carrying the sum's identity in its
// back-pointer.
func Sum(path, name string, variants ...string) *node.Sum {
	held := make([]*node.SumVariant, 0, len(variants))
	for _, variant := range variants {
		held = append(held, &node.SumVariant{
			ID:   MemberID(path, name, variant, symbol.KindSumVariant),
			Name: variant,
			Host: ID(path, name, symbol.KindSum),
		})
	}
	return &node.Sum{
		ID:         ID(path, name, symbol.KindSum),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Variants:   held,
	}
}

// Variable returns a package-level variable declaration.
func Variable(path, name string) *node.Variable {
	return &node.Variable{
		ID:         ID(path, name, symbol.KindVariable),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Mutability: symbol.MutabilityMutable,
	}
}

// Constant returns a package-level constant declaration.
func Constant(path, name string) *node.Constant {
	return &node.Constant{
		ID:         ID(path, name, symbol.KindConstant),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
	}
}

// Interface returns an interface declaring one method, under the
// interface's name.
func Interface(path, name string) *node.Interface {
	m := Method(path, name, MethodName)
	m.Host = ID(path, name, symbol.KindInterface)
	m.Abstract = true
	return &node.Interface{
		ID:         ID(path, name, symbol.KindInterface),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Methods:    []*node.Method{m},
	}
}

// Alias returns a type alias naming its target by spelling alone,
// as an unlinked reference arrives.
func Alias(path, name string) *node.Alias {
	return &node.Alias{
		ID:         ID(path, name, symbol.KindAlias),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Target:     &node.TypeRef{Spelling: StructName},
	}
}

// StructName is the struct the every-kind fixture hangs its members
// on, and the target [Alias] points at.
const StructName = "Row"

// Populated returns a struct carrying one field and one method, so
// a traversal that descends into members meets both.
func Populated(path, name string) *node.Struct {
	s := Struct(path, name)
	s.Fields = []*node.Field{Field(path, name, FieldName)}
	s.Methods = []*node.Method{Method(path, name, MethodName)}
	return s
}

// EveryKind returns a package holding one declaration of every kind
// a rule can match: the twelve kinds the generated constructors
// cover, with the member kinds hanging on their hosts rather than
// sitting loose in the file, and the function's parameter and
// return on its signature.
//
// A case over the whole vocabulary uses this rather than naming
// kinds one at a time, because a fixture that omits a kind reports
// as a passing test over a rule that never fired.
func EveryKind(path string) *node.Package {
	return Package(path,
		Populated(path, StructName),
		Interface(path, InterfaceName),
		Enum(path, EnumName, EnumVariantName),
		Sum(path, SumName, SumVariantName),
		Alias(path, AliasName),
		Function(path, FunctionName),
		Variable(path, VariableName),
		Constant(path, ConstantName),
	)
}

// EveryKindID returns the identity [EveryKind] assigns its
// declaration of one kind under name: a field or a method under
// [StructName], a parameter or a return under [FunctionName], a
// variant under its enum or sum, and anything else at the top
// level.
func EveryKindID(path, name string, kind symbol.Kind) symbol.Identity {
	switch kind {
	case symbol.KindField, symbol.KindMethod:
		return MemberID(path, StructName, name, kind)
	case symbol.KindParam, symbol.KindReturn:
		return MemberID(path, FunctionName, name, kind)
	case symbol.KindEnumVariant:
		return MemberID(path, EnumName, name, kind)
	case symbol.KindSumVariant:
		return MemberID(path, SumName, name, kind)
	default:
		return ID(path, name, kind)
	}
}

// MatchableKinds are the kinds [EveryKind] declares, which is the
// set the generated match constructors cover. A case asserting that
// it reached the whole vocabulary compares against this.
func MatchableKinds() []symbol.Kind {
	return []symbol.Kind{
		symbol.KindStruct,
		symbol.KindField,
		symbol.KindMethod,
		symbol.KindInterface,
		symbol.KindEnum,
		symbol.KindSum,
		symbol.KindAlias,
		symbol.KindFunction,
		symbol.KindParam,
		symbol.KindReturn,
		symbol.KindVariable,
		symbol.KindConstant,
	}
}

// KindCount returns how many declarations of one kind the packages
// hold.
//
// It counts what the store indexes: a declaration that names itself
// and carries an identity. A case dispatching over the same packages
// compares its invocations against this rather than a written-out
// number, so a fixture that grows a kind does not quietly weaken the
// case.
func KindCount(kind symbol.Kind, pkgs ...*node.Package) int {
	count := 0
	for _, pkg := range pkgs {
		for decl := range node.Declarations(pkg) {
			if decl.Kind() == kind && !decl.Identity().IsZero() {
				count++
			}
		}
	}
	return count
}

// Foreign returns a declaration that names a kind and an identity
// without being the node model's type for that kind, as a symbol
// another model placed in a declaration list would be.
//
// The store indexes it under the kind it names, so a rule of that
// kind is dispatched to it and the rule's own type assertion refuses
// it. Nothing built from the model's kinds reaches that guard.
func Foreign(path, name string, kind symbol.Kind) node.Declaration {
	return foreign{id: ID(path, name, kind)}
}

// foreign is [Foreign]'s declaration: an identity, the kind that
// identity names, and nothing the node model would recognize.
type foreign struct{ id symbol.Identity }

func (f foreign) Kind() symbol.Kind         { return f.id.Kind }
func (foreign) Position() position.Pos      { return position.Pos{} }
func (foreign) Docs() []string              { return nil }
func (f foreign) Identity() symbol.Identity { return f.id }

// ID returns the identity the resolution step assigns a top-level
// declaration of that kind in one package.
//
// Every fixture identity is built here or in [MemberID], so a case
// comparing two of them compares the same spelling rules rather
// than two hand-built literals that agree by luck.
func ID(path, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: Lang, Package: path, Name: name, Kind: kind}
}

// MemberID returns the identity the resolution step assigns a
// member declaration: owner is the dotted chain of the enclosing
// declarations' names, so two hosts' members of one name spell
// apart.
func MemberID(path, owner, name string, kind symbol.Kind) symbol.Identity {
	id := ID(path, name, kind)
	id.Owner = owner
	return id
}
