// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
)

// resolvePolicies is the policy step. It returns the resolved lowering
// policy of each plan, at the plan's index, and the backends whose
// policies are defective, each mapped to true. A plan's selection takes
// the choices of Config.Policies for the keys that the plan's backend
// declares. The plan's own PlanConfig.Policies replaces each of these
// choices that it selects again. A plan without a backend has the zero
// policy, because the plan step refuses it.
//
// It returns one error for each of these faults:
//   - Config.Policies selects a key that no plan's backend declares. The
//     error lists every key that the backends declare.
//   - A plan's selection has a key that the plan's backend does not
//     declare. The error lists the backend's keys.
//   - A selection has a choice outside the choices of its key. The error
//     lists the choices.
//   - A backend declares a defective policy. The error contains the
//     defect as [plugin.NewPolicy] returns it, once for each backend.
//
// Where the backends do not declare a policy and the config does not
// select one, the step allocates only the list of policies.
func resolvePolicies(plans []Plan, cfg Config) ([]plugin.Policy, map[plugin.ID]bool, []error) {
	var faults []error
	out := make([]plugin.Policy, len(plans))
	// checked maps each backend that declares policies to whether they
	// are defective.
	var checked map[plugin.ID]bool
	var declared []string
	for i, pl := range plans {
		b := pl.Backend
		if b == nil {
			continue
		}
		specs := policiesOf(b)
		defective, done := checked[b.Name()]
		if !done && len(specs) > 0 {
			for _, s := range specs {
				declared = append(declared, string(s.Key))
			}
			_, err := plugin.NewPolicy(b.Target(), specs, nil)
			if err != nil {
				faults = append(faults, fmt.Errorf(
					"workspace: backend %s declares a defective policy: %w", b.Name(), err,
				))
			}
			if checked == nil {
				checked = map[plugin.ID]bool{}
			}
			defective = err != nil
			checked[b.Name()] = defective
		}
		if defective {
			continue
		}
		var chosen map[plugin.PolicyKey]plugin.Choice
		if own := cfg.Plans[pl.Name].Policies; len(cfg.Policies)+len(own) > 0 {
			chosen = make(map[plugin.PolicyKey]plugin.Choice, len(own))
			maps.Copy(chosen, own)
			for _, s := range specs {
				c, selected := cfg.Policies[s.Key]
				if _, mine := own[s.Key]; selected && !mine {
					chosen[s.Key] = c
				}
			}
		}
		policy, err := plugin.NewPolicy(b.Target(), specs, chosen)
		if err != nil {
			faults = append(faults, fmt.Errorf("workspace: plan %q: %w", pl.Name, err))
			continue
		}
		out[i] = policy
	}
	if len(cfg.Policies) == 0 {
		return out, checked, faults
	}
	slices.Sort(declared)
	declared = slices.Compact(declared)
	for _, k := range slices.Sorted(maps.Keys(cfg.Policies)) {
		if slices.Contains(declared, string(k)) {
			continue
		}
		if len(declared) == 0 {
			faults = append(faults, fmt.Errorf(
				"workspace: the config selects policy %s, and no plan's backend declares a policy", k,
			))
			continue
		}
		faults = append(faults, fmt.Errorf(
			"workspace: the config selects policy %s, which no plan's backend declares: the backends declare %s",
			k, strings.Join(declared, ", "),
		))
	}
	return out, checked, faults
}
