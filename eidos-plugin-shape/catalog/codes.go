// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog

import "go.dokimi.dev/eidos/sdk/diag"

// Prefix is the prefix of the codes that the plugins of the catalog report
// under.
const Prefix diag.Prefix = "SHAPE"

var (
	// RoleArity reports a role of a contract instance whose number of
	// callables its arity does not admit, at a callable of the instance.
	RoleArity = diag.MustRegister(Prefix, diag.CodeSpec{
		Number:  1,
		Meaning: "a role of a contract instance has a number of callables that its arity does not admit",
	})
	// ParamRange reports an int param of a directive below the minimum of
	// its spec, at the directive.
	ParamRange = diag.MustRegister(Prefix, diag.CodeSpec{
		Number:  2,
		Meaning: "an int param of a classification's directive is below the minimum of its spec",
	})
	// ExclusiveParams reports a directive that writes two params that its
	// spec excludes from each other, at the directive.
	ExclusiveParams = diag.MustRegister(Prefix, diag.CodeSpec{
		Number:  3,
		Meaning: "a classification's directive writes two params that its spec excludes from each other",
	})
	// UnsharedParam reports a directive whose host-param reference resolves
	// to a parameter that the callable of another param does not declare,
	// at the directive.
	UnsharedParam = diag.MustRegister(Prefix, diag.CodeSpec{
		Number:  4,
		Meaning: "the callable of a param lacks the parameter that a host-param reference of the directive resolves to",
	})
)
