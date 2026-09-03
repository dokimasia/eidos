// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The boolean spellings validation accepts, exactly.
const (
	spellTrue  = "true"
	spellFalse = "false"
)

// Resolver binds one source reference param: what a spelling
// names from a subject, at a resolution kind. The workspace
// derives it from the registered rules and a view minted over the
// sealed graph for validation; the view's reads record into a set
// the run discards, because validation runs whole on every run. An
// error names what was looked for and not found.
type Resolver func(subject symbol.Identity, name string, kind ResolutionKind) (symbol.Identity, error)

// Validate types and checks every instance on one subject,
// reporting each violation as a positioned Error on sink and
// returning the instances that passed, in position order, with
// repeatable instances numbered.
//
// keys resolves ResolveMetadataKey params; resolve binds every
// other reference kind, and nil carries those spellings unbound.
// Validation of one subject is independent of every other, so a
// caller validates subjects in parallel; the sink is safe for
// that, and the resolver is called from every goroutine. It
// refuses an unsealed registry outright: that is a defect in the
// composition, not in a carrier.
func Validate(
	subject symbol.Identity, ds []Raw,
	r *Registry, keys *meta.Registry, resolve Resolver, sink *diag.Sink,
) []Directive {
	if len(ds) == 0 {
		return nil
	}
	v := &validator{registry: r, keys: keys, resolve: resolve, sink: sink, subject: subject}
	if !r.Sealed() {
		v.report(UnsealedRegistry, ds[0].Pos,
			"directives on %s validate before the registry sealed", subject)
		return nil
	}

	ordered := slices.Clone(ds)
	slices.SortFunc(ordered, func(a, b Raw) int { return comparePos(a.Pos, b.Pos) })

	// Type each instance alone, keeping its schema for the
	// subject-wide checks below.
	typed := make([]checked, 0, len(ordered))
	for _, raw := range ordered {
		if instance, schema, ok := v.instance(raw); ok {
			typed = append(typed, checked{instance: instance, schema: schema})
		}
	}

	// The subject-wide checks: repeatability, then the constraints
	// between directives. A failing instance drops; the survivors
	// number per schema in position order.
	present := map[Name][]int{}
	for i, c := range typed {
		canonical := c.schema.Canonical()
		present[canonical] = append(present[canonical], i)
	}
	dropped := make([]bool, len(typed))
	for canonical, indexes := range present {
		if len(indexes) > 1 && !typed[indexes[0]].schema.Repeatable {
			first := typed[indexes[0]].instance.Pos
			for _, i := range indexes[1:] {
				v.reportRelated(DuplicateInstance, typed[i].instance.Pos, first,
					"%s appears twice on %s: the schema admits one instance", canonical, subject)
			}
			for _, i := range indexes {
				dropped[i] = true
			}
		}
	}
	for i, c := range typed {
		for _, required := range c.schema.Requires {
			target, _ := v.registry.ResolveName(required)
			if _, held := present[target.Canonical()]; !held {
				v.report(RequirementUnmet, c.instance.Pos,
					"%s requires %s, which %s does not carry",
					c.schema.Canonical(), required, subject)
				dropped[i] = true
			}
		}
		for _, conflicting := range c.schema.ConflictsWith {
			target, _ := v.registry.ResolveName(conflicting)
			others, held := present[target.Canonical()]
			if !held {
				continue
			}
			for _, other := range others {
				v.reportRelated(Conflict, c.instance.Pos, typed[other].instance.Pos,
					"%s conflicts with %s on %s",
					c.schema.Canonical(), target.Canonical(), subject)
				dropped[i] = true
				dropped[other] = true
			}
		}
	}

	counters := map[Name]int{}
	out := make([]Directive, 0, len(typed))
	for i, c := range typed {
		if dropped[i] {
			continue
		}
		c.instance.Instance = counters[c.instance.Name]
		counters[c.instance.Name]++
		out = append(out, c.instance)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// checked pairs a typed instance with the schema that typed it.
type checked struct {
	instance Directive
	schema   Schema
}

// validator carries the registries and the sink through one
// subject's validation. at is the instance under validation, so a
// value-level refusal reports where its carrier sits.
type validator struct {
	registry *Registry
	keys     *meta.Registry
	resolve  Resolver
	sink     *diag.Sink
	subject  symbol.Identity
	at       position.Pos
}

// report attaches one positioned Error.
func (v *validator) report(code diag.Code, at position.Pos, format string, args ...any) {
	v.sink.Errorf(code, at, diag.PhaseFreeze, format, args...)
}

// reportRelated attaches one positioned Error carrying the other
// position the finding is about.
func (v *validator) reportRelated(
	code diag.Code, at, other position.Pos, format string, args ...any,
) {
	v.sink.Report(diag.Diag{
		Code:     code,
		Severity: diag.SeverityError,
		Pos:      at,
		Msg:      fmt.Sprintf(format, args...),
		Origin:   diag.PhaseFreeze,
		Related:  []position.Pos{other},
	})
}

// instance types one raw instance against its schema. A false
// answer means the violations are reported and the instance drops.
func (v *validator) instance(raw Raw) (Directive, Schema, bool) {
	v.at = raw.Pos
	schema, held := v.registry.ResolveName(raw.Name)
	if !held {
		if v.registry.Ignored(raw.Name) {
			// The workspace opted out: a foreign tool's carrier drops
			// without a finding.
			return Directive{}, Schema{}, false
		}
		if candidates := v.registry.Candidates(raw.Name); len(candidates) > 1 {
			v.report(AmbiguousName, raw.Pos,
				"%s has two claimants: write one of %s", raw.Name, nameList(candidates))
		} else {
			v.report(UnclaimedName, raw.Pos, "%s names no registered schema", raw.Name)
		}
		return Directive{}, Schema{}, false
	}

	d := Directive{Name: schema.Canonical(), Params: map[ParamKey]Value{}, Pos: raw.Pos}
	ok := true
	positional := 0
	seen := map[ParamKey]int{}
	for _, arg := range raw.Args {
		if arg.Key == "" {
			if positional >= len(schema.Positional) {
				v.report(ExtraPositional, raw.Pos,
					"%s takes %d positional arguments; %q at offset %d is one too many",
					d.Name, len(schema.Positional), rawSpelling(arg.Value), arg.Col)
				ok = false
				continue
			}
			spec := schema.Positional[positional]
			positional++
			value, typedOK := v.typed(d.Name, spec, arg)
			if !typedOK {
				ok = false
				continue
			}
			d.Args = append(d.Args, value)
			continue
		}

		key := ParamKey(arg.Key)
		if first, twice := seen[key]; twice {
			v.report(DuplicateKey, raw.Pos,
				"%s writes %s twice, at offsets %d and %d", d.Name, key, first, arg.Col)
			ok = false
			continue
		}
		seen[key] = arg.Col

		if key == roleKey {
			role, roleOK := v.role(d.Name, schema, arg)
			if !roleOK {
				ok = false
				continue
			}
			d.Role = role
			continue
		}
		spec, declared := findParam(schema, key)
		reserved := key == ReservedOut || key == ReservedTag
		switch {
		case declared:
		case reserved:
			spec = ParamSpec{Key: key, Type: TypeString}
		case schema.Open != nil:
			spec = *schema.Open
			spec.Key = key
		default:
			v.report(UnknownKey, raw.Pos,
				"%s does not accept %s", d.Name, key)
			ok = false
			continue
		}
		value, typedOK := v.typed(d.Name, spec, arg)
		if !typedOK {
			ok = false
			continue
		}
		d.Params[key] = value
	}

	// The role gates which params were legal and which are owed,
	// so its checks run after every argument is read.
	if schema.RolesRequired && d.Role == "" {
		v.report(MissingRole, raw.Pos,
			"%s demands a role, one of %s", d.Name, strings.Join(schema.Roles, ", "))
		ok = false
	}
	for key := range d.Params {
		spec, declared := findParam(schema, key)
		if !declared && schema.Open != nil && key != ReservedOut && key != ReservedTag {
			spec, declared = *schema.Open, true
		}
		if !declared {
			continue // a reserved key is admitted under every role
		}
		if !roleAdmits(spec.Roles, d.Role) {
			v.report(UnknownKey, raw.Pos,
				"%s does not accept %s under role %q", d.Name, key, d.Role)
			ok = false
		}
	}
	for i, spec := range schema.Positional {
		if spec.Required && i >= len(d.Args) {
			v.report(MissingParam, raw.Pos,
				"%s omits %s, which its schema requires", d.Name, spec.Key)
			ok = false
		}
	}
	for _, spec := range schema.Params {
		_, given := d.Params[spec.Key]
		if spec.Required && !given && roleAdmits(spec.Roles, d.Role) {
			v.report(MissingParam, raw.Pos,
				"%s omits %s, which its schema requires", d.Name, spec.Key)
			ok = false
		}
	}

	if !ok {
		return Directive{}, Schema{}, false
	}
	return d, schema, true
}

// role validates the role argument against the schema's declared
// set.
func (v *validator) role(name Name, schema Schema, arg RawArg) (string, bool) {
	if len(schema.Roles) == 0 {
		v.report(UnknownKey, v.at,
			"%s declares no roles, so it does not accept %s", name, roleKey)
		return "", false
	}
	if arg.Value.List != nil {
		v.report(TypeMismatch, v.at, "%s takes one role, not a list", name)
		return "", false
	}
	role := arg.Value.Text
	if !slices.Contains(schema.Roles, role) {
		v.report(UnknownRole, v.at,
			"%s does not declare role %q: one of %s",
			name, role, strings.Join(schema.Roles, ", "))
		return "", false
	}
	return role, true
}

// typed types one argument's value against its spec.
func (v *validator) typed(name Name, spec ParamSpec, arg RawArg) (Value, bool) {
	return v.typedValue(name, spec, spec.Type, arg.Value)
}

// typedValue types one raw value as t, recursing into list
// elements with the element type.
func (v *validator) typedValue(name Name, spec ParamSpec, t ParamType, raw RawValue) (Value, bool) {
	if t == TypeList {
		if raw.List == nil {
			v.report(TypeMismatch, v.at,
				"%s param %s takes a list", name, spec.Key)
			return Value{}, false
		}
		out := Value{Kind: TypeList, List: make([]Value, 0, len(raw.List))}
		for _, element := range raw.List {
			typed, ok := v.typedValue(name, spec, spec.ListOf, element)
			if !ok {
				return Value{}, false
			}
			out.List = append(out.List, typed)
		}
		return out, true
	}

	if raw.List != nil {
		v.report(TypeMismatch, v.at,
			"%s param %s takes a single value, not a list", name, spec.Key)
		return Value{}, false
	}
	switch t {
	case TypeString:
		return Value{Kind: TypeString, Str: raw.Text}, true
	case TypeInt:
		parsed, err := strconv.ParseInt(raw.Text, 10, 64)
		if err != nil {
			v.report(BadSpelling, v.at,
				"%s param %s takes a base-10 integer, not %q", name, spec.Key, raw.Text)
			return Value{}, false
		}
		return Value{Kind: TypeInt, Int: parsed}, true
	case TypeBool:
		switch raw.Text {
		case spellTrue:
			return Value{Kind: TypeBool, Bool: true}, true
		case spellFalse:
			return Value{Kind: TypeBool, Bool: false}, true
		}
		v.report(BadSpelling, v.at,
			"%s param %s takes exactly %s or %s, not %q",
			name, spec.Key, spellTrue, spellFalse, raw.Text)
		return Value{}, false
	case TypeReference:
		if spec.Resolution == ResolveMetadataKey {
			if !v.metadataResolves(raw.Text) {
				v.report(UnknownMetadataKey, v.at,
					"%s param %s names %q, which no metadata key or group returns; keys: %s",
					name, spec.Key, raw.Text, v.metadataCandidates())
				return Value{}, false
			}
			return Value{Kind: TypeReference, Ref: raw.Text}, true
		}
		if v.resolve == nil {
			return Value{Kind: TypeReference, Ref: raw.Text}, true
		}
		target, err := v.resolve(v.subject, raw.Text, spec.Resolution)
		if err != nil {
			v.report(UnresolvedReference, v.at,
				"%s param %s names %q, which does not resolve as %s: %v",
				name, spec.Key, raw.Text, spec.Resolution, err)
			return Value{}, false
		}
		return Value{Kind: TypeReference, Ref: raw.Text, Target: target}, true
	}
	v.report(TypeMismatch, v.at, "%s param %s has no type", name, spec.Key)
	return Value{}, false
}

// metadataResolves reports whether a spelling names a registered
// key or a fact group.
func (v *validator) metadataResolves(spelling string) bool {
	if _, held := v.keys.Resolve(meta.KeyName(spelling)); held {
		return true
	}
	members := v.keys.Group(meta.GroupName(spelling))
	for range members {
		return true
	}
	return false
}

// metadataCandidates spells the registered keys for the refusal.
func (v *validator) metadataCandidates() string {
	var out []string
	for name := range v.keys.Keys() {
		out = append(out, string(name))
	}
	return strings.Join(out, ", ")
}

// findParam returns a schema's keyed spec.
func findParam(s Schema, key ParamKey) (ParamSpec, bool) {
	for _, spec := range s.Params {
		if spec.Key == key {
			return spec, true
		}
	}
	return ParamSpec{}, false
}

// roleAdmits reports whether a param's role scope admits an
// instance's role. An empty scope admits every role, the empty one
// included.
func roleAdmits(scope []string, role string) bool {
	return len(scope) == 0 || role != "" && slices.Contains(scope, role)
}

// nameList spells candidate names for a refusal.
func nameList(names []Name) string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, string(n))
	}
	return strings.Join(out, ", ")
}

// rawSpelling returns a raw value's spelling for a refusal.
func rawSpelling(v RawValue) string {
	if v.List == nil {
		return v.Text
	}
	return "a list"
}

// comparePos orders positions by file, line then column.
func comparePos(a, b position.Pos) int {
	return cmp.Or(
		strings.Compare(a.File, b.File),
		cmp.Compare(a.Line, b.Line),
		cmp.Compare(a.Col, b.Col),
	)
}
