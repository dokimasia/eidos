// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/layout"
)

// Policy is a layout policy. In a config file, a policy is one of the names
// that [layout.Policy.String] returns: inherit, alongside-source or
// centralised.
type Policy layout.Policy

// UnmarshalYAML decodes a policy from n with [layout.ParsePolicy].
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n when
// n is not the name of a policy.
func (p *Policy) UnmarshalYAML(n *yaml.Node) error {
	parsed, err := layout.ParsePolicy(n.Value)
	if err != nil {
		return lineFault(n, "%q is not a layout policy: use %s", n.Value, policyNames())
	}
	*p = Policy(parsed)
	return nil
}

// schema returns the JSON Schema of a policy. The schema allows the name of
// each policy.
func (Policy) schema() map[string]any {
	var names []any
	for q := layout.PolicyInherit; q.Valid(); q++ {
		names = append(names, q.String())
	}
	return map[string]any{"type": "string", "enum": names}
}

// policyNames joins the names of the policies for an error message, as in
// "inherit, alongside-source or centralised".
func policyNames() string {
	var out string
	for q := layout.PolicyInherit; q.Valid(); q++ {
		switch {
		case q == layout.PolicyInherit:
			out = q.String()
		case (q + 1).Valid():
			out += ", " + q.String()
		default:
			out += " or " + q.String()
		}
	}
	return out
}
