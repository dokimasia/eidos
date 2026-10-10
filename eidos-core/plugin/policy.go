// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/directive"
)

// The separators of the policy's spellings: the dot between the target's
// spelling and the policy's name in a [PolicyKey], and the comma between
// the values that a message lists.
const (
	policySeparator = "."
	listSeparator   = ", "
)

// PolicyKey is the key of one lowering policy of a target. It is the
// target's spelling, a dot and the policy's name, as in
// typescript.int64. The param of the target's directive with the
// policy's name overrides the policy for one declaration, as in
// typescript int64=string. The override is a fact under the metadata key
// with the same spelling as the PolicyKey.
type PolicyKey string

// Param returns the param of the target's directive that overrides the
// policy. The param is the policy's name, as int64 is for
// typescript.int64. A key without a dot has the empty param.
func (k PolicyKey) Param() directive.ParamKey {
	_, name, _ := strings.Cut(string(k), policySeparator)
	return directive.ParamKey(name)
}

// Choice is one value that a policy takes, such as bigint.
type Choice string

// PolicySpec declares one lowering policy. Default is the choice that
// applies where nothing selects one.
type PolicySpec struct {
	Key     PolicyKey
	Choices []Choice
	Default Choice
	// Doc is a sentence about what the policy decides. It is the
	// documentation of the policy's metadata key and of the directive
	// param that overrides the policy.
	Doc string
}

// Policy is a resolved lowering policy: one choice for every key that a
// target declares. The zero Policy has no keys.
//
// # Concurrency
//
// A Policy is immutable after [NewPolicy] returns it, so the plans of a
// run read one policy concurrently. A copy that [Policy.Overridden]
// returns is as safe for concurrent use as its override function.
//
// # Allocation contract
//
// [Policy.Choice] and [Policy.Overridden] do not allocate. A call of the
// override function allocates what the function allocates.
type Policy struct {
	specs    []PolicySpec // sorted by key
	chosen   []Choice     // the choice of each spec, in the same order
	override func(PolicyKey) (Choice, bool)
}

// NewPolicy returns the resolved policy of the target t. A key takes the
// choice that chosen selects for it, and the default of its spec where
// chosen selects none. NewPolicy checks each spec as the backend kit's
// Build checks it.
//
// Error modes, one error for each fault, joined:
//   - a spec with an empty key, a key outside t's namespace, or a key
//     that another spec declares;
//   - a policy name that is not a valid param key of a directive, and the
//     policy name name, which the target's directive keeps for a
//     declaration's name;
//   - a spec without choices, a spec with an empty or a repeated choice,
//     and a spec whose default is not one of its choices;
//   - a spec without documentation;
//   - a key of chosen that no spec declares, with an error that contains
//     the declared keys where t declares any;
//   - a choice outside its key's choices, with an error that contains
//     them.
//
// # Allocation contract
//
// NewPolicy allocates the policy's sorted copy of the specs and its list
// of choices, and the sorted keys of chosen where chosen has a key. Each
// fault allocates its error.
func NewPolicy(t Target, specs []PolicySpec, chosen map[PolicyKey]Choice) (Policy, error) {
	sorted := slices.Clone(specs)
	slices.SortFunc(sorted, func(a, b PolicySpec) int { return strings.Compare(string(a.Key), string(b.Key)) })
	var faults []error
	resolved := make([]Choice, len(sorted))
	for i, s := range sorted {
		if err := checkSpec(t, s); err != nil {
			faults = append(faults, err)
		}
		if i > 0 && sorted[i-1].Key == s.Key {
			faults = append(faults, fmt.Errorf("plugin: target %s declares policy %s twice", t, s.Key))
		}
		resolved[i] = s.Default
	}
	if len(chosen) > 0 {
		for _, k := range slices.Sorted(maps.Keys(chosen)) {
			i, declared := slices.BinarySearchFunc(sorted, k, func(s PolicySpec, k PolicyKey) int {
				return strings.Compare(string(s.Key), string(k))
			})
			if !declared && len(sorted) == 0 {
				faults = append(faults,
					fmt.Errorf("plugin: target %s does not declare a policy, and %s is selected", t, k))
				continue
			}
			if !declared {
				keys := make([]PolicyKey, 0, len(sorted))
				for _, s := range sorted {
					keys = append(keys, s.Key)
				}
				faults = append(faults, fmt.Errorf(
					"plugin: target %s does not declare the policy %s: its policies are %s", t, k, joined(keys)))
				continue
			}
			if !slices.Contains(sorted[i].Choices, chosen[k]) {
				faults = append(faults, fmt.Errorf("plugin: policy %s takes one of %s, not %q",
					k, joined(sorted[i].Choices), chosen[k]))
				continue
			}
			resolved[i] = chosen[k]
		}
	}
	if len(faults) > 0 {
		return Policy{}, errors.Join(faults...)
	}
	return Policy{specs: sorted, chosen: resolved}, nil
}

// Choice returns the choice of a key: the override's choice where the
// policy has an override that reports one, and the resolved choice
// otherwise. It panics for a key that the policy does not declare,
// because a spoke reads only the keys of its own backend.
func (p Policy) Choice(k PolicyKey) Choice {
	i, declared := slices.BinarySearchFunc(p.specs, k, func(s PolicySpec, k PolicyKey) int {
		return strings.Compare(string(s.Key), string(k))
	})
	if !declared {
		panic("plugin: the policy does not declare the key " + string(k) +
			", and a spoke reads only the keys of its own backend")
	}
	if p.override != nil {
		if c, overridden := p.override(k); overridden {
			return c
		}
	}
	return p.chosen[i]
}

// Overridden returns a copy of the policy whose Choice asks override
// first. The authoring kit passes the reader of one translation, which
// reads the key's value at directive authority on the declaration. The
// copy shares the specs and the choices, so Overridden allocates
// nothing. A nil override returns a copy without an override.
func (p Policy) Overridden(override func(PolicyKey) (Choice, bool)) Policy {
	p.override = override
	return p
}

// checkSpec returns the first defect of one spec of the target t, and
// nil for a spec without one.
func checkSpec(t Target, s PolicySpec) error {
	if s.Key == "" {
		return fmt.Errorf("plugin: target %s declares a policy without a key", t)
	}
	prefix, _, dotted := strings.Cut(string(s.Key), policySeparator)
	if !dotted || Target(prefix) != t {
		return fmt.Errorf("plugin: policy %s is outside the namespace of target %s", s.Key, t)
	}
	param := s.Key.Param()
	if !param.Valid() {
		return fmt.Errorf("plugin: policy %s has a name that is not a valid param key of a directive", s.Key)
	}
	if param == NameParam {
		return fmt.Errorf("plugin: policy %s takes the name %s, which the target's directive keeps for a name",
			s.Key, NameParam)
	}
	if len(s.Choices) == 0 {
		return fmt.Errorf("plugin: policy %s does not declare a choice", s.Key)
	}
	for i, c := range s.Choices {
		if c == "" {
			return fmt.Errorf("plugin: policy %s declares an empty choice", s.Key)
		}
		if slices.Contains(s.Choices[:i], c) {
			return fmt.Errorf("plugin: policy %s declares choice %s twice", s.Key, c)
		}
	}
	if !slices.Contains(s.Choices, s.Default) {
		return fmt.Errorf("plugin: policy %s defaults to %q, which is not one of %s",
			s.Key, s.Default, joined(s.Choices))
	}
	if s.Doc == "" {
		return fmt.Errorf("plugin: policy %s has no documentation", s.Key)
	}
	return nil
}

// joined returns the spellings of values separated by commas, for a
// message.
func joined[T ~string](values []T) string {
	var b strings.Builder
	for i, v := range values {
		if i > 0 {
			b.WriteString(listSeparator)
		}
		b.WriteString(string(v))
	}
	return b.String()
}
