// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
	"errors"
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
	// ignored holds the spellings the workspace opted out of
	// reporting: a full name, or a plugin prefix with its colon.
	ignored map[Name]bool
	// constraints maps each schema to its Requires and ConflictsWith
	// as canonical spellings. The seal fills it, so validation
	// resolves nothing per instance.
	constraints map[Name]resolved
	sealed      bool
}

// resolved is one schema's constraints as canonical spellings.
type resolved struct {
	requires  []Name
	conflicts []Name
}

// NewRegistry returns a registry holding nothing.
func NewRegistry() *Registry {
	return &Registry{
		byCanonical: map[Name]Schema{},
		claimants:   map[Name][]Name{},
		ignored:     map[Name]bool{},
		constraints: map[Name]resolved{},
	}
}

// Ignore records a spelling whose unclaimed instances validation
// drops in silence: a foreign tool's carriers living in the same
// comments. A full name ignores that directive. A plugin prefix
// ending in its colon, as in "k8s:", ignores every directive under
// it. Ignoring a claimed name is refused, at the call when the
// schema registered first and at the seal otherwise, because
// silencing a registered directive would hide its validation.
func (r *Registry) Ignore(n Name) error {
	if r.sealed {
		return fmt.Errorf("directive: ignoring %s after the seal: registration ends there", n)
	}
	if n == "" || n == Name(prefixSep) {
		return errors.New("directive: an empty spelling ignores nothing")
	}
	if slices.Contains(kernelNames, n) {
		return fmt.Errorf("directive: %s is a kernel name, which no workspace ignores", n)
	}
	if err := r.claimedIgnore(n); err != nil {
		return err
	}
	r.ignored[n] = true
	return nil
}

// Ignored reports whether a spelling is opted out: by its full
// name, or by the prefix of the plugin it names.
func (r *Registry) Ignored(n Name) bool {
	if r.ignored[n] {
		return true
	}
	plugin, _, prefixed := strings.Cut(string(n), string(prefixSep))
	return prefixed && r.ignored[Name(plugin+string(prefixSep))]
}

// Register records one schema.
//
// It refuses: a registration after the seal, a kernel name claimed
// by a plugin, an empty Plugin on any name outside the kernel's
// six, a name, plugin prefix or param key the grammar cannot spell,
// a reserved key among the params, a param key or role declared
// twice, a positional param carrying Roles, an undeclared role on a
// param, a role requirement on a schema declaring no roles, a list
// of lists, a list stating no element type, a reference stating no
// resolution, an untyped param, and an empty doc anywhere. One
// plugin claiming one name twice is refused naming both docs; two
// plugins claiming one bare name both register, and the bare
// spelling becomes ambiguous.
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
// registered, refuses an ignore covering a registered name, and
// closes the registry. Each unknown name is one error, a
// self-reference another, a silenced schema another; composition
// collects them all.
func (r *Registry) Seal() []error {
	var faults []error
	for _, n := range r.ignoredOrder() {
		if err := r.claimedIgnore(n); err != nil {
			faults = append(faults, err)
		}
	}
	for _, canonical := range r.canonicalOrder() {
		s := r.byCanonical[canonical]
		var c resolved
		for i, constraint := range [2][]Name{s.Requires, s.ConflictsWith} {
			for _, named := range constraint {
				_, target, held := r.lookup(named)
				if !held {
					faults = append(faults, fmt.Errorf(
						"directive: %s constrains %q, which nothing registered or two claim",
						canonical, named,
					))
					continue
				}
				if target == canonical {
					faults = append(faults, fmt.Errorf(
						"directive: %s constrains itself", canonical,
					))
				}
				if i == 0 {
					c.requires = append(c.requires, target)
				} else {
					c.conflicts = append(c.conflicts, target)
				}
			}
		}
		r.constraints[canonical] = c
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
	s, _, held := r.lookup(n)
	return s, held
}

// Candidates returns every canonical spelling that claims a bare
// name, in registration order, for the ambiguity Error.
func (r *Registry) Candidates(n Name) []Name {
	return slices.Clone(r.claimants[n])
}

// lookup returns the schema a spelling addresses together with its
// canonical spelling, which is the key it is held under, so no
// caller spells the canonical form again.
func (r *Registry) lookup(n Name) (Schema, Name, bool) {
	if s, held := r.byCanonical[n]; held {
		return s, n, true
	}
	claimants := r.claimants[n]
	if len(claimants) != 1 {
		return Schema{}, "", false
	}
	return r.byCanonical[claimants[0]], claimants[0], true
}

// constraintsOf returns a schema's constraints as the seal resolved
// them.
func (r *Registry) constraintsOf(canonical Name) resolved { return r.constraints[canonical] }

// claimedIgnore refuses an ignore that would silence a registered
// schema: the name itself, a bare name any plugin claims, or a
// prefix covering one. A bare name two plugins claim is refused as
// well, so validation reports each of its instances as ambiguous.
func (r *Registry) claimedIgnore(n Name) error {
	if _, held := r.byCanonical[n]; held || len(r.claimants[n]) > 0 {
		return fmt.Errorf("directive: ignoring %s would silence its registered schema", n)
	}
	if !strings.HasSuffix(string(n), string(prefixSep)) {
		return nil
	}
	for _, canonical := range r.canonicalOrder() {
		if strings.HasPrefix(string(canonical), string(n)) {
			return fmt.Errorf(
				"directive: ignoring %s would silence %s, whose schema is registered", n, canonical,
			)
		}
	}
	return nil
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
	// A carrier writes the name, the prefix and every key through the
	// grammar's ident, so a spelling outside it registers a schema no
	// author can address.
	if !isIdentifier(string(s.Name)) {
		return fmt.Errorf("directive: %q is no name a carrier can spell", s.Name)
	}
	if s.Plugin != "" && !isIdentifier(s.Plugin) {
		return fmt.Errorf("directive: %s names plugin %q, which no carrier can spell as a prefix",
			s.Name, s.Plugin)
	}
	for _, spec := range slices.Concat(s.Positional, s.Params) {
		if !isIdentifier(string(spec.Key)) {
			return fmt.Errorf("directive: %s param %q is no key a carrier can spell", s.Name, spec.Key)
		}
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
	if s.Open != nil {
		return admissibleOpen(s, *s.Open, roles)
	}
	return nil
}

// admissibleOpen checks the spec an open schema types its
// undeclared keys with: it names no key of its own, and is
// otherwise a param.
func admissibleOpen(s Schema, spec ParamSpec, roles map[string]struct{}) error {
	if spec.Key != "" {
		return fmt.Errorf(
			"directive: %s opens under key %q: an open spec names no key, the instance's does",
			s.Name, spec.Key,
		)
	}
	spec.Key = openKey
	return admissibleParam(s, spec, map[ParamKey]struct{}{}, roles)
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
	if spec.Type == TypeList && spec.ListOf == 0 {
		return fmt.Errorf("directive: %s param %q states no element type for its list", s.Name, spec.Key)
	}
	references := spec.Type == TypeReference || spec.Type == TypeList && spec.ListOf == TypeReference
	if references && spec.Resolution == ResolveNone {
		return fmt.Errorf("directive: %s param %q references and states no resolution kind", s.Name, spec.Key)
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

// ignoredOrder returns every ignored spelling, sorted, so the
// seal's faults arrive in one order.
func (r *Registry) ignoredOrder() []Name {
	out := make([]Name, 0, len(r.ignored))
	for n := range r.ignored {
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b Name) int { return strings.Compare(string(a), string(b)) })
	return out
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
