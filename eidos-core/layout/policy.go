// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"strconv"
)

// Policy is where a plan places a generated file by default.
type Policy uint8

const (
	// PolicyInherit uses the policy of the enclosing scope. A refinement
	// uses the policy of its plan, and a plan uses PolicyAlongside.
	PolicyInherit Policy = 0
	// PolicyAlongside places a file in the directory of its source.
	PolicyAlongside Policy = 1
	// PolicyCentralised places a file under the configured output
	// directory, in the same relative directory as its source package.
	PolicyCentralised Policy = 2
)

// ParsePolicy returns the policy whose name is s. The names are the strings
// that [Policy.String] returns: inherit, alongside-source and centralised.
// The comparison is exact, so "Centralised" is not a policy. ParsePolicy
// allocates nothing when s is the name of a policy.
//
// Error modes: an error that quotes s and lists the three names, when s is
// not one of them.
func ParsePolicy(s string) (Policy, error) {
	for p := PolicyInherit; p.Valid(); p++ {
		if p.String() == s {
			return p, nil
		}
	}
	return PolicyInherit, fmt.Errorf("layout: %q is not a layout policy: use %s, %s or %s",
		s, PolicyInherit, PolicyAlongside, PolicyCentralised)
}

// Valid reports whether p is one of the three policies.
func (p Policy) Valid() bool { return p <= PolicyCentralised }

// String returns the name of the policy in a configuration, such as
// "centralised". For a value outside the three policies, it returns the
// number in the form Policy(n).
func (p Policy) String() string {
	switch p {
	case PolicyInherit:
		return "inherit"
	case PolicyAlongside:
		return "alongside-source"
	case PolicyCentralised:
		return "centralised"
	default:
		return "Policy(" + strconv.Itoa(int(p)) + ")"
	}
}

// or returns p, or fallback when p is PolicyInherit.
func (p Policy) or(fallback Policy) Policy {
	if p == PolicyInherit {
		return fallback
	}
	return p
}
