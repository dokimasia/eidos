// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape

import (
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Scoped is the match of a function or a method. It reads facts as every
// matcher does, and it reads declarations through the tracked reader of
// its invocation. [sdk.FunctionMatch] and [sdk.MethodMatch] satisfy it.
type Scoped interface {
	sdk.Matcher
	Reader() *store.Reader
}

// Instance is one instance of a contract. The contract, the package of its
// callables and the id identify it.
type Instance struct {
	Contract Contract
	// Package is the package that declares the callables of the instance.
	Package symbol.Identity
	// ID is empty where the package has one instance of the contract.
	ID string
	// Members are the functions and the methods of the instance with their
	// roles, in the order of their declarations.
	Members []Member
}

// Member is one callable of a contract instance and its role.
type Member struct {
	Callable symbol.Identity
	Role     Role
}

// InstanceOf returns the instance of the contract c that a callable has a
// role in. It returns false for a callable without a role in c, and for a
// name that no contract spec has. The callable is the function or the
// method of m. The members of the instance are the functions and the
// methods of the package of the callable that have a role in c and the
// same id. The methods of two types of one package therefore join one
// instance, unless their ids differ. A callable whose package the view of
// m does not contain has an instance without members.
//
// InstanceOf reads the package of the callable through the tracked reader
// of m, and the role and the id of each callable of the package through
// m. A change of the package or of the facts of a callable of the package
// runs the handler again.
//
// # Allocation contract
//
// InstanceOf allocates the growth of the list of members, three
// allocations for three members, and its reads allocate as [sdk.FactOf]
// states.
func InstanceOf(m Scoped, callable node.Declaration, c Contract) (Instance, bool) {
	s, _ := SpecOf(c)
	role, id := meta.Named[string](s.Key), meta.Named[string](s.ID)
	self := callable.Identity()
	if _, held := sdk.FactOf(m, self, role); !held {
		return Instance{}, false
	}
	inst := Instance{Contract: c, Package: self.PackageIdentity()}
	inst.ID, _ = sdk.FactOf(m, self, id)
	pkg, held := m.Reader().PackageOf(self)
	if !held {
		return inst, true
	}
	for decl := range node.Declarations(pkg) {
		if kind := decl.Kind(); kind != symbol.KindFunction && kind != symbol.KindMethod {
			continue
		}
		member := decl.Identity()
		r, inRole := sdk.FactOf(m, member, role)
		if !inRole {
			continue
		}
		if memberID, _ := sdk.FactOf(m, member, id); memberID != inst.ID {
			continue
		}
		inst.Members = append(inst.Members, Member{Callable: member, Role: Role(r)})
	}
	return inst, true
}
