// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/symbol"
)

// Registry records every schema a workspace recognises.
//
// An open Registry is not safe for concurrent use. Registration happens
// while the workspace composes, and sealing ends it: validation
// refuses an unsealed registry, and registration after the seal is
// an error. A sealed Registry changes on no call, so it is safe for
// concurrent use.
type Registry struct {
	// byCanonical maps each canonical spelling to its schema.
	byCanonical map[Name]Schema
	// claimants maps each bare name to the canonical spellings that
	// claim it, in registration order.
	claimants map[Name][]Name
	// ignored records the spellings the workspace opted out of
	// reporting: a full name, or a plugin prefix with its colon.
	ignored map[Name]bool
	// constraints maps each schema to its Requires and ConflictsWith
	// as canonical spellings. The seal fills it, so validation
	// resolves nothing per instance.
	constraints map[Name]resolved
	// targets records the names of the targets' directives, which the
	// registry treats as kernel names. RegisterTarget creates it.
	targets map[Name]bool
	sealed  bool
}

// resolved is one schema's constraints as canonical spellings.
type resolved struct {
	requires  []Name
	conflicts []Name
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byCanonical: map[Name]Schema{},
		claimants:   map[Name][]Name{},
		ignored:     map[Name]bool{},
		constraints: map[Name]resolved{},
	}
}

// Ignore records a spelling whose unclaimed instances validation
// drops in silence: a foreign tool's carriers in the same comments.
// A full name ignores that directive. A plugin prefix
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
	if slices.Contains(kernelNames, n) || r.targets[n] {
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
	plugin := n.Plugin()
	return plugin != "" && r.ignored[Name(plugin+string(prefixSep))]
}

// Register records one schema. It returns an error for:
//   - a registration after the seal;
//   - a kernel name claimed by a plugin, and an empty Plugin on any
//     name outside the kernel's six. A target's name that
//     [Registry.RegisterTarget] recorded is a kernel name;
//   - a negatable or override kernel schema, because a negated instance
//     and an override instance opt a subject out of one plugin's rules
//     and the kernel is no plugin;
//   - a name, plugin prefix or param key the grammar cannot spell;
//   - a reserved key among the params, and a param key or role
//     declared twice;
//   - a positional param with Roles, an undeclared role on a param,
//     and a role requirement on a schema that declares no roles;
//   - roles beside variants, a variant name the grammar cannot spell
//     or that the schema declares twice, and a variant param with the
//     key of a schema param;
//   - a list of lists, a list without an element type, and a
//     reference without a resolution kind;
//   - choices on a param that is not a string, an empty choice, and a
//     choice declared twice;
//   - a default on a required, list or reference param, and a default
//     that does not type as the param's type;
//   - a value of Kinds that is no declaration kind, [symbol.KindInvalid]
//     included, and a kind that Kinds lists twice;
//   - an untyped param, and an empty doc anywhere;
//   - a canonical spelling registered twice, with an error naming
//     both docs.
//
// A variant's params and roles are checked as a schema's are.
//
// Two plugins that claim one bare name both register, and the bare
// spelling becomes ambiguous.
func (r *Registry) Register(s Schema) error {
	if r.sealed {
		return fmt.Errorf("directive: %s registers after the seal: registration ends there", s.Name)
	}
	if err := r.admissible(s); err != nil {
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

// RegisterTarget records the directive of one rendering target. The
// schema has no plugin, and its name is the target's spelling, such as
// typescript. The kernel's lowering entry reads its instances. From the
// registration on, the registry treats the name as a kernel name, so it
// refuses a plugin's schema of the name and an ignore of it. The
// workspace registers each target's directive after the kernel's
// schemas and before any plugin's.
//
// Error modes: a registration after the seal, a schema with a plugin, a
// target registered twice, one of the kernel's six names, a name that a
// registered schema claims, and every refusal of the schema's own
// declaration that [Registry.Register] makes.
func (r *Registry) RegisterTarget(s Schema) error {
	if r.sealed {
		return fmt.Errorf("directive: target %s registers after the seal: registration ends there", s.Name)
	}
	if s.Plugin != "" {
		return fmt.Errorf("directive: target %s has the plugin %q, and the directive of a target has no plugin",
			s.Name, s.Plugin)
	}
	if r.targets[s.Name] {
		return fmt.Errorf("directive: target %s is registered twice", s.Name)
	}
	if slices.Contains(kernelNames, s.Name) {
		return fmt.Errorf("directive: target %s is a kernel name, which no target takes", s.Name)
	}
	if claimants := r.claimants[s.Name]; len(claimants) > 0 {
		return fmt.Errorf("directive: target %s is a name that %s claims", s.Name, claimants[0])
	}
	if r.targets == nil {
		r.targets = map[Name]bool{}
	}
	r.targets[s.Name] = true
	if err := r.admissible(s); err != nil {
		delete(r.targets, s.Name)
		return err
	}
	r.byCanonical[s.Name] = s
	r.claimants[s.Name] = append(r.claimants[s.Name], s.Name)
	return nil
}

// Seal resolves every Requires and ConflictsWith against what
// registered, refuses an ignore covering a registered name, and
// closes the registry. Each unknown name, self-reference and
// silenced schema is one error, and composition collects them all.
// Seal on a sealed registry returns nothing and leaves the registry as
// it is, so a sealed registry changes on no call and is safe for
// concurrent use.
func (r *Registry) Seal() []error {
	if r.sealed {
		return nil
	}
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
// A prefixed spelling addresses its plugin's schema. A bare
// spelling addresses the schema when exactly one plugin claims it.
// With two claimants it returns false, and [Registry.Candidates]
// names them for the diagnostic.
func (r *Registry) ResolveName(n Name) (Schema, bool) {
	s, _, held := r.lookup(n)
	return s, held
}

// Candidates returns every canonical spelling that claims a bare
// name, in registration order, for the ambiguity Error.
func (r *Registry) Candidates(n Name) []Name {
	return slices.Clone(r.claimants[n])
}

// Schemas returns each registered schema in the order of its canonical
// spelling, the schemas of the kernel and of the targets included. A
// schema shares its lists with the declaration that registered it, so a
// caller does not change them. A documentation tool reads the schemas of
// a composition through it.
func (r *Registry) Schemas() iter.Seq[Schema] {
	return func(yield func(Schema) bool) {
		for _, canonical := range r.canonicalOrder() {
			if !yield(r.byCanonical[canonical]) {
				return
			}
		}
	}
}

// lookup returns the schema a spelling addresses together with its
// canonical spelling, which is the key it is stored under, so no
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

// The bounds of a suggestion: a registered name within suggestEdits edits
// of a spelling, and within a third of the length of the longer of the
// two, is near.
const (
	suggestEdits   = 2
	suggestPortion = 3
)

// nearest returns the registered name nearest to a spelling that names no
// schema: a canonical name for a spelling with a plugin prefix, and a bare
// name otherwise. It reports false where no name is near. Two names at one
// distance return the first in name order, so the suggestion does not
// depend on the order of registration.
func (r *Registry) nearest(n Name) (Name, bool) {
	var best Name
	bestDistance := suggestEdits + 1
	consider := func(candidate Name) {
		d := editDistance(string(n), string(candidate))
		if d*suggestPortion > max(len(n), len(candidate)) {
			return
		}
		if d < bestDistance || d == bestDistance && candidate < best {
			best, bestDistance = candidate, d
		}
	}
	if n.Plugin() != "" {
		for canonical := range r.byCanonical {
			consider(canonical)
		}
	} else {
		for bare := range r.claimants {
			consider(bare)
		}
	}
	return best, bestDistance <= suggestEdits
}

// editDistance returns the optimal string alignment distance between two
// spellings: the count of byte insertions, deletions, substitutions and
// swaps of two adjacent bytes that turn a into b, where no byte is edited
// twice. It allocates three rows of the length of b, and only a refusal
// calls it.
func editDistance(a, b string) int {
	before, prev, cur := make([]int, len(b)+1), make([]int, len(b)+1), make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				cur[j] = min(cur[j], before[j-2]+1)
			}
		}
		before, prev, cur = prev, cur, before
	}
	return prev[len(b)]
}

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

// admissible checks one schema's own declaration. A name of the kernel
// is one of its six names or a target's name that RegisterTarget
// recorded.
func (r *Registry) admissible(s Schema) error {
	kernel := slices.Contains(kernelNames, s.Name) || r.targets[s.Name]
	if s.Plugin == "" && !kernel {
		return fmt.Errorf(
			"directive: %s names no plugin, and only the kernel's own schemas may", s.Name,
		)
	}
	if s.Plugin != "" && kernel {
		return fmt.Errorf("directive: %s is a kernel name, which no plugin claims", s.Name)
	}
	if s.Plugin == "" && s.Negatable {
		return fmt.Errorf(
			"directive: %s is a kernel schema, and only a plugin's schema is negatable", s.Name,
		)
	}
	if s.Plugin == "" && s.Overrides {
		return fmt.Errorf(
			"directive: %s is a kernel schema, and only a plugin's schema is an override schema", s.Name,
		)
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
	for _, specs := range [2][]ParamSpec{s.Positional, s.Params} {
		for _, spec := range specs {
			if !isIdentifier(string(spec.Key)) {
				return fmt.Errorf("directive: %s param %q is no key a carrier can spell", s.Name, spec.Key)
			}
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
	if len(s.Roles) > 0 && len(s.Variants) > 0 {
		return fmt.Errorf("directive: %s declares roles beside variants, and only a variant declares roles", s.Name)
	}
	for i, k := range s.Kinds {
		if _, declared := symbol.ParseKind(k.String()); !declared {
			return fmt.Errorf("directive: %s lists %s, which is no declaration kind", s.Name, k)
		}
		if slices.Contains(s.Kinds[:i], k) {
			return fmt.Errorf("directive: %s lists the kind %s twice", s.Name, k)
		}
	}
	roles := map[string]struct{}{}
	if err := declareRoles(string(s.Name), s.Roles, roles); err != nil {
		return err
	}

	keys := map[ParamKey]struct{}{}
	for _, spec := range s.Positional {
		if err := admissibleParam(s, string(s.Name), spec, keys, roles); err != nil {
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
		if err := admissibleParam(s, string(s.Name), spec, keys, roles); err != nil {
			return err
		}
	}
	names := map[string]struct{}{}
	for _, v := range s.Variants {
		if err := admissibleVariant(s, v, keys, names); err != nil {
			return err
		}
	}
	if s.Open != nil {
		return admissibleOpen(s, *s.Open, roles)
	}
	return nil
}

// admissibleVariant checks one variant's declaration and claims its
// name in names. The variant's params join the schema's on an instance,
// so schemaKeys contains the keys that no variant param may have.
func admissibleVariant(s Schema, v Variant, schemaKeys map[ParamKey]struct{}, names map[string]struct{}) error {
	if !isIdentifier(v.Name) {
		return fmt.Errorf("directive: %s variant %q is no name a carrier can spell", s.Name, v.Name)
	}
	if _, taken := names[v.Name]; taken {
		return fmt.Errorf("directive: %s declares variant %s twice", s.Name, v.Name)
	}
	names[v.Name] = struct{}{}
	owner := string(s.Name) + " variant " + v.Name
	if v.Doc == "" {
		return fmt.Errorf("directive: %s states no semantics", owner)
	}
	if v.RolesRequired && len(v.Roles) == 0 {
		return fmt.Errorf("directive: %s demands a role and declares none to choose from", owner)
	}
	roles := map[string]struct{}{}
	if err := declareRoles(owner, v.Roles, roles); err != nil {
		return err
	}
	keys := map[ParamKey]struct{}{}
	for _, spec := range v.Params {
		if !isIdentifier(string(spec.Key)) {
			return fmt.Errorf("directive: %s param %q is no key a carrier can spell", owner, spec.Key)
		}
		if _, taken := schemaKeys[spec.Key]; taken {
			return fmt.Errorf("directive: %s declares param %q, which the schema declares for every variant",
				owner, spec.Key)
		}
		if err := admissibleParam(s, owner, spec, keys, roles); err != nil {
			return err
		}
	}
	return nil
}

// declareRoles adds each role of a schema or a variant to roles. owner
// is the schema or the variant that an error states. It returns an error
// for a role declared twice.
func declareRoles(owner string, declared []string, roles map[string]struct{}) error {
	for _, role := range declared {
		if _, taken := roles[role]; taken {
			return fmt.Errorf("directive: %s declares role %q twice", owner, role)
		}
		roles[role] = struct{}{}
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
	return admissibleParam(s, string(s.Name), spec, map[ParamKey]struct{}{}, roles)
}

// admissibleParam checks one param's declaration and claims its key.
// owner is the schema or the variant that declares the param, as its
// errors state it, and roles contains the roles of the owner.
func admissibleParam(
	s Schema, owner string, spec ParamSpec, keys map[ParamKey]struct{}, roles map[string]struct{},
) error {
	if spec.Key == roleKey {
		return fmt.Errorf("directive: %s claims the reserved key %q: declaring Roles is what reserves it",
			owner, spec.Key)
	}
	// The reserved routing keys belong to the kernel: its own
	// schemas may declare them, a plugin's may not.
	if s.Plugin != "" && (spec.Key == ReservedOut || spec.Key == ReservedTag) {
		return fmt.Errorf("directive: %s claims the reserved key %q", owner, spec.Key)
	}
	if spec.Type == 0 {
		return fmt.Errorf("directive: %s param %q states no type", owner, spec.Key)
	}
	if spec.Doc == "" {
		return fmt.Errorf("directive: %s param %q states no semantics", owner, spec.Key)
	}
	if len(spec.Choices) > 0 && spec.Type != TypeString {
		return fmt.Errorf("directive: %s param %q declares choices, and only a string param takes them",
			owner, spec.Key)
	}
	for i, choice := range spec.Choices {
		if choice == "" {
			return fmt.Errorf("directive: %s param %q declares an empty choice", owner, spec.Key)
		}
		if slices.Contains(spec.Choices[:i], choice) {
			return fmt.Errorf("directive: %s param %q declares choice %q twice", owner, spec.Key, choice)
		}
	}
	if spec.Default != "" {
		if spec.Required {
			return fmt.Errorf("directive: %s param %q declares a default and requires the param", owner, spec.Key)
		}
		if spec.Type != TypeString && spec.Type != TypeInt && spec.Type != TypeBool {
			return fmt.Errorf("directive: %s param %q declares a default, and only a string, integer or "+
				"boolean param takes one", owner, spec.Key)
		}
		if _, reason := scalar(spec, spec.Type, spec.Default); reason != "" {
			return fmt.Errorf("directive: %s param %q declares a default that does not type: %s",
				owner, spec.Key, reason)
		}
	}
	if spec.Type == TypeList && spec.ListOf == TypeList {
		return fmt.Errorf(
			"directive: %s param %q nests a list in a list: a directive that "+
				"needs structure splits in two", owner, spec.Key,
		)
	}
	if spec.Type == TypeList && spec.ListOf == 0 {
		return fmt.Errorf("directive: %s param %q states no element type for its list", owner, spec.Key)
	}
	references := spec.Type == TypeReference || spec.Type == TypeList && spec.ListOf == TypeReference
	if references && spec.Resolution == ResolveNone {
		return fmt.Errorf("directive: %s param %q references and states no resolution kind", owner, spec.Key)
	}
	if _, taken := keys[spec.Key]; taken {
		return fmt.Errorf("directive: %s declares param %q twice", owner, spec.Key)
	}
	keys[spec.Key] = struct{}{}
	for _, role := range spec.Roles {
		if _, declared := roles[role]; !declared {
			return fmt.Errorf(
				"directive: %s scopes param %q to role %q, which it does not declare",
				owner, spec.Key, role,
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
