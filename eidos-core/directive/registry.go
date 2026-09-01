// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
	"fmt"
	"slices"
	"strings"
)

// Registry holds every schema a workspace recognises.
//
// A Registry is not safe for concurrent use. Registration happens
// while the workspace composes, and sealing ends it: validation
// refuses an unsealed registry, and registration after the seal is
// an error.
type Registry struct {
	// byCanonical holds every schema under its canonical spelling.
	byCanonical map[Name]Schema
	// claimants holds, per bare name, the canonical spellings that
	// claim it, in registration order.
	claimants map[Name][]Name
	sealed    bool
}

// NewRegistry returns a registry holding nothing.
func NewRegistry() *Registry {
	return &Registry{
		byCanonical: map[Name]Schema{},
		claimants:   map[Name][]Name{},
	}
}

// Register records one schema.
//
// It refuses: a registration after the seal, a kernel name claimed
// by a plugin, an empty Plugin on any name outside the kernel's
// four, a reserved key among the params, a param key or role
// declared twice, a positional param carrying Roles, an undeclared
// role on a param, a role requirement on a schema declaring no
// roles, a list of lists, an untyped param, and an empty doc
// anywhere. One plugin claiming one name twice is refused naming
// both docs; two plugins claiming one bare name both register, and
// the bare spelling becomes ambiguous.
func (r *Registry) Register(s Schema) error {
	if r.sealed {
		return fmt.Errorf("directive: %s registers after the seal: registration ends there", s.Name)
	}
	if err := admissible(s); err != nil {
		return err
	}

	canonical := s.Canonical()
	if held, taken := r.byCanonical[canonical]; taken {
		return fmt.Errorf("directive: %s is registered twice: %q and %q",
			canonical, held.Doc, s.Doc)
	}
	r.byCanonical[canonical] = s
	r.claimants[s.Name] = append(r.claimants[s.Name], canonical)
	return nil
}

// Seal resolves every Requires and ConflictsWith against what
// registered and closes the registry. Each unknown name is one
// error, a self-reference is another; composition collects them
// all.
func (r *Registry) Seal() []error {
	var faults []error
	for _, canonical := range r.canonicalOrder() {
		s := r.byCanonical[canonical]
		for _, constraint := range [2][]Name{s.Requires, s.ConflictsWith} {
			for _, named := range constraint {
				target, held := r.ResolveName(named)
				if !held {
					faults = append(faults, fmt.Errorf(
						"directive: %s constrains %q, which nothing registered or two claim",
						canonical, named,
					))
					continue
				}
				if target.Canonical() == canonical {
					faults = append(faults, fmt.Errorf(
						"directive: %s constrains itself", canonical,
					))
				}
			}
		}
	}
	r.sealed = true
	return faults
}

// Sealed reports whether registration has ended.
func (r *Registry) Sealed() bool { return r.sealed }

// ResolveName returns the schema a spelling addresses.
//
// A prefixed spelling addresses its owner's schema. A bare
// spelling addresses the schema iff exactly one plugin claims it;
// with two claimants it returns false, and Candidates names them
// for the diagnostic.
func (r *Registry) ResolveName(n Name) (Schema, bool) {
	if s, held := r.byCanonical[n]; held {
		return s, true
	}
	claimants := r.claimants[n]
	if len(claimants) != 1 {
		return Schema{}, false
	}
	return r.byCanonical[claimants[0]], true
}

// Candidates returns every canonical spelling that claims a bare
// name, in registration order, for the ambiguity Error.
func (r *Registry) Candidates(n Name) []Name {
	return slices.Clone(r.claimants[n])
}

// admissible checks one schema's own declaration.
func admissible(s Schema) error {
	kernel := slices.Contains(kernelNames, s.Name)
	if s.Plugin == "" && !kernel {
		return fmt.Errorf(
			"directive: %s names no plugin, and only the kernel's own schemas may", s.Name,
		)
	}
	if s.Plugin != "" && kernel {
		return fmt.Errorf("directive: %s is a kernel name, which no plugin claims", s.Name)
	}
	if s.Doc == "" {
		return fmt.Errorf("directive: %s states no semantics: the schema is its documentation", s.Name)
	}
	if s.RolesRequired && len(s.Roles) == 0 {
		return fmt.Errorf(
			"directive: %s demands a role and declares none to choose from", s.Name,
		)
	}
	roles := map[string]struct{}{}
	for _, role := range s.Roles {
		if _, taken := roles[role]; taken {
			return fmt.Errorf("directive: %s declares role %q twice", s.Name, role)
		}
		roles[role] = struct{}{}
	}

	keys := map[ParamKey]struct{}{}
	for _, spec := range s.Positional {
		if err := admissibleParam(s, spec, keys, roles); err != nil {
			return err
		}
		if len(spec.Roles) > 0 {
			return fmt.Errorf(
				"directive: %s scopes positional %q by role, which would shift positions",
				s.Name, spec.Key,
			)
		}
	}
	for _, spec := range s.Params {
		if err := admissibleParam(s, spec, keys, roles); err != nil {
			return err
		}
	}
	return nil
}

// admissibleParam checks one param's declaration and claims its
// key.
func admissibleParam(
	s Schema, spec ParamSpec, keys map[ParamKey]struct{}, roles map[string]struct{},
) error {
	if spec.Key == roleKey {
		return fmt.Errorf("directive: %s claims the reserved key %q: declaring Roles is what reserves it",
			s.Name, spec.Key)
	}
	// The reserved routing keys belong to the kernel: its own
	// schemas may declare them, a plugin's may not.
	if s.Plugin != "" && (spec.Key == ReservedOut || spec.Key == ReservedTag) {
		return fmt.Errorf("directive: %s claims the reserved key %q", s.Name, spec.Key)
	}
	if spec.Type == 0 {
		return fmt.Errorf("directive: %s param %q states no type", s.Name, spec.Key)
	}
	if spec.Doc == "" {
		return fmt.Errorf("directive: %s param %q states no semantics", s.Name, spec.Key)
	}
	if spec.Type == TypeList && spec.ListOf == TypeList {
		return fmt.Errorf(
			"directive: %s param %q nests a list in a list: a directive that "+
				"needs structure splits in two", s.Name, spec.Key,
		)
	}
	if _, taken := keys[spec.Key]; taken {
		return fmt.Errorf("directive: %s declares param %q twice", s.Name, spec.Key)
	}
	keys[spec.Key] = struct{}{}
	for _, role := range spec.Roles {
		if _, declared := roles[role]; !declared {
			return fmt.Errorf(
				"directive: %s param %q is scoped to role %q, which the schema does not declare",
				s.Name, spec.Key, role,
			)
		}
	}
	return nil
}

// canonicalOrder returns every canonical spelling, sorted, so the
// seal's faults arrive in one order.
func (r *Registry) canonicalOrder() []Name {
	out := make([]Name, 0, len(r.byCanonical))
	for name := range r.byCanonical {
		out = append(out, name)
	}
	slices.SortFunc(out, func(a, b Name) int { return strings.Compare(string(a), string(b)) })
	return out
}
