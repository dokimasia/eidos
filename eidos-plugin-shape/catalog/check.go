// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// CheckID is the name of the plugin that validates contract instances.
const CheckID plugin.ID = "shapecheck"

// newChecker returns the plugin shapecheck. It declares one rule for each
// contract, behind a gate on the family key of the contract. The handler
// of the rule validates the instance of its callable and stamps nothing.
// The plugin requires [shape.Capability], so the workspace runs it after
// the plugin shape.
func newChecker(specs []shape.Spec) plugin.Annotator {
	var rs []sdk.Rule
	for i := range specs {
		s := &specs[i]
		if s.Form != shape.FormContract {
			continue
		}
		rs = append(rs, sdk.Where(shape.In(shape.Contract(s.Name)),
			sdk.OnFunction(func(m *sdk.FunctionMatch, _ *sdk.Stamper) error {
				validate(m, s, m.Function)
				return nil
			}),
			sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Stamper) error {
				validate(m, s, m.Method)
				return nil
			}),
		))
	}
	a, _ := sdk.NewPlugin(CheckID).Requires(shape.Capability).Handle(rs...).Build().(plugin.Annotator)
	return a
}

// validate checks the arity of every role of the instance of a callable.
// A role with more callables than its arity admits reports at each
// callable of the role. A role without a callable that its arity requires
// reports once, at the first member of the instance. The callable is the
// subject of the match, so its package is in the view and the instance
// has the callable as a member.
func validate(m match, s *shape.Spec, callable node.Declaration) {
	inst, _ := shape.InstanceOf(m, callable, shape.Contract(s.Name))
	self := callable.Identity()
	var own shape.Role
	count := make(map[shape.Role]int, len(s.Roles))
	for _, member := range inst.Members {
		count[member.Role]++
		if member.Callable == self {
			own = member.Role
		}
	}
	what := "the contract " + s.Name + " in the package " + inst.Package.Package
	if inst.ID != "" {
		what = "the instance " + inst.ID + " of the contract " + s.Name + " in the package " + inst.Package.Package
	}
	for _, r := range s.Roles {
		n := count[r.Name]
		switch {
		case r.Arity.Admits(n):
		case n > 1 && own == r.Name:
			m.Errorf(RoleArity, "%s has %d callables in the role %s, whose arity is %s", what, n, r.Name, r.Arity)
		case n == 0 && inst.Members[0].Callable == self:
			m.Errorf(RoleArity, "%s has no callable in the role %s, whose arity is %s", what, r.Name, r.Arity)
		}
	}
}
