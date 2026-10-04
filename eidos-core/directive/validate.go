// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
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
// sealed graph for validation. The view's reads record into a set
// the run discards, because validation runs whole on every run. An
// error names what was looked for and not found.
type Resolver func(subject symbol.Identity, name string, kind ResolutionKind) (symbol.Identity, error)

// Validate types and checks every instance on one subject,
// reporting each violation as a positioned Error on sink and
// returning the instances that passed, in position order, with
// repeatable instances numbered.
//
// A negated instance types like any other. It is refused where its
// schema is not [Schema.Negatable], and it takes part in no
// requirement and no conflict, because it withdraws the subject
// from a plugin and states nothing the constraints read. A subject
// that sets and negates one directive states two opposite intents,
// and every instance of that directive on it is refused.
//
// A repeatable directive whose instances on the subject mix carriers
// in the tool-directive shape with other carriers reports a Warning
// under [MixedCarriers] and keeps every instance, because a
// formatter that moves the shaped lines, as gofmt does, reorders the
// instances and their numbering.
//
// keys resolves ResolveMetadataKey params. resolve binds every
// other reference kind, and a nil resolver leaves those spellings
// unbound. Validation of one subject is independent of every other,
// so a caller validates subjects in parallel: the sink is safe for
// concurrent use, and the resolver is called from every goroutine.
// Validate refuses an unsealed registry outright, because that is a
// defect in the composition, not in a carrier.
//
// # Allocation contract
//
// Validate allocates what it returns: the slice of instances, and per
// instance its params map with the map's one group, each param's value,
// which the map stores apart from the group because a [Value] is larger
// than 128 bytes, the positional arguments, and each list. A subject
// with more than four instances, and an instance with more than eight
// params, grow the working storage onto the heap. A finding allocates
// what the sink does.
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

	// The store seals a subject's instances in position order, so a
	// copy is made only for a caller that passes them out of order.
	ordered := ds
	if !slices.IsSortedFunc(ds, comparePos) {
		ordered = slices.Clone(ds)
		slices.SortFunc(ordered, comparePos)
	}

	// Type each instance alone, keeping its schema for the
	// subject-wide checks below. A typed instance's Name is its
	// schema's canonical spelling. A subject has few instances, so
	// they type into storage on the stack.
	var scratch [typedScratch]checked
	typed := scratch[:0]
	for _, raw := range ordered {
		if instance, schema, ok := v.instance(raw); ok {
			typed = append(typed, checked{instance: instance, schema: schema, shaped: raw.DirectiveShaped})
		}
	}

	// The subject-wide checks: polarity and repeatability, then the
	// constraints between directives. A failing instance drops, and
	// the survivors number per schema in position order. Every loop
	// runs in position order, so the findings arrive in one order. A
	// directive's instances are checked at its first instance, those
	// that set it apart from those that negate it.
	var setsScratch, negationsScratch [typedScratch]int
	dropped := make([]bool, len(typed))
	for i, c := range typed {
		canonical := c.instance.Name
		if slices.ContainsFunc(typed[:i], func(earlier checked) bool { return earlier.instance.Name == canonical }) {
			continue
		}
		sets := instancesOf(setsScratch[:0], typed, canonical, false)
		negations := instancesOf(negationsScratch[:0], typed, canonical, true)
		if len(sets) > 0 && len(negations) > 0 {
			first := typed[sets[0]].instance.Pos
			for _, i := range negations {
				v.reportRelated(diag.SeverityError, Conflict, typed[i].instance.Pos, first,
					"%s is set and negated on %s", canonical, subject)
			}
			for _, i := range sets {
				dropped[i] = true
			}
			for _, i := range negations {
				dropped[i] = true
			}
			continue
		}
		for _, indexes := range [2][]int{sets, negations} {
			if len(indexes) < 2 {
				continue
			}
			if typed[indexes[0]].schema.Repeatable {
				v.mixedCarriers(typed, indexes, canonical)
				continue
			}
			first := typed[indexes[0]].instance.Pos
			for _, i := range indexes[1:] {
				v.reportRelated(diag.SeverityError, DuplicateInstance, typed[i].instance.Pos, first,
					"%s appears twice on %s: the schema admits one instance", canonical, subject)
			}
			for _, i := range indexes {
				dropped[i] = true
			}
		}
	}
	// A conflict is symmetric, so a pair both schemas declare is one
	// contradiction and reports once.
	reported := map[[2]int]bool{}
	for i, c := range typed {
		if c.instance.Negated {
			continue
		}
		constraints := v.registry.constraintsOf(c.instance.Name)
		for _, target := range constraints.requires {
			if len(instancesOf(setsScratch[:0], typed, target, false)) == 0 {
				v.report(RequirementUnmet, c.instance.Pos,
					"%s requires %s, which %s does not have",
					c.instance.Name, target, subject)
				dropped[i] = true
			}
		}
		for _, target := range constraints.conflicts {
			for _, other := range instancesOf(setsScratch[:0], typed, target, false) {
				dropped[i] = true
				dropped[other] = true
				pair := [2]int{min(i, other), max(i, other)}
				if reported[pair] {
					continue
				}
				reported[pair] = true
				v.reportRelated(diag.SeverityError, Conflict, c.instance.Pos, typed[other].instance.Pos,
					"%s conflicts with %s on %s", c.instance.Name, target, subject)
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

// typedScratch is how many instances of one subject, and how many
// instances of one directive on it, validation keeps on the stack. A
// subject with more grows onto the heap.
const typedScratch = 4

// paramScratch is how many params of one instance validation orders on
// the stack. An instance with more grows onto the heap.
const paramScratch = 8

// checked pairs a typed instance with the schema that typed it, and
// with whether its carrier line has the tool-directive shape.
type checked struct {
	instance Directive
	schema   Schema
	shaped   bool
}

// instancesOf appends to dst the indexes of the typed instances of the
// canonical name that negate it, or that set it where negated is false,
// in position order.
func instancesOf(dst []int, typed []checked, canonical Name, negated bool) []int {
	for i, c := range typed {
		if c.instance.Name == canonical && c.instance.Negated == negated {
			dst = append(dst, i)
		}
	}
	return dst
}

// validator keeps the registries and the sink for one subject's
// validation. at is the instance under validation, so a
// value-level refusal reports at its carrier's line.
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

// reportRelated attaches one positioned finding at the severity,
// with the other position the finding is about.
func (v *validator) reportRelated(
	severity diag.Severity, code diag.Code, at, other position.Pos, format string, args ...any,
) {
	v.sink.Report(diag.Diag{
		Code:     code,
		Severity: severity,
		Pos:      at,
		Msg:      fmt.Sprintf(format, args...),
		Origin:   diag.PhaseFreeze,
		Related:  []position.Pos{other},
	})
}

// mixedCarriers warns where the instances of one repeatable
// directive mix carriers in the tool-directive shape with other
// carriers. It reports once, at the first instance whose shape
// differs from the first instance's, and drops nothing.
func (v *validator) mixedCarriers(typed []checked, indexes []int, canonical Name) {
	first := typed[indexes[0]]
	for _, i := range indexes[1:] {
		if typed[i].shaped == first.shaped {
			continue
		}
		v.reportRelated(diag.SeverityWarning, MixedCarriers, typed[i].instance.Pos, first.instance.Pos,
			"%s on %s mixes carriers in the tool-directive form with other carriers, "+
				"which a formatter may reorder: write every instance in one form", canonical, v.subject)
		return
	}
}

// instance types one raw instance against its schema. It reports
// false when it reported a violation, and the instance drops.
func (v *validator) instance(raw Raw) (Directive, Schema, bool) {
	v.at = raw.Pos
	schema, canonical, held := v.registry.lookup(raw.Name)
	if !held {
		// An ambiguous spelling reports even where an ignore covers
		// it: the registry refuses such an ignore, and a claimed name
		// is never a foreign tool's.
		if candidates := v.registry.Candidates(raw.Name); len(candidates) > 1 {
			v.report(AmbiguousName, raw.Pos,
				"%s has two claimants: write one of %s", raw.Name, nameList(candidates))
			return Directive{}, Schema{}, false
		}
		if v.registry.Ignored(raw.Name) {
			// The workspace opted out: a foreign tool's carrier drops
			// without a finding.
			return Directive{}, Schema{}, false
		}
		v.report(UnclaimedName, raw.Pos, "%s names no registered schema", raw.Name)
		return Directive{}, Schema{}, false
	}
	if raw.Negated && !schema.Negatable {
		v.report(NegationRefused, raw.Pos,
			"%s does not accept the negated form: its schema declares no negation", canonical)
		return Directive{}, Schema{}, false
	}

	d := Directive{Name: canonical, Params: map[ParamKey]Value{}, Pos: raw.Pos, Negated: raw.Negated}
	ok := true
	positional := 0
	seen := map[ParamKey]int{}
	for _, arg := range raw.Args {
		if arg.Key == "" {
			if positional >= len(schema.Positional) {
				v.report(ExtraPositional, raw.Pos,
					"%s takes %d positional arguments, and %q at offset %d is one too many",
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
	// so its checks run after every argument is read. Keys check in
	// key order, so the findings arrive in one order.
	if schema.RolesRequired && d.Role == "" {
		v.report(MissingRole, raw.Pos,
			"%s demands a role, one of %s", d.Name, strings.Join(schema.Roles, ", "))
		ok = false
	}
	var keysScratch [paramScratch]ParamKey
	keys := keysScratch[:0]
	for key := range d.Params {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
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
	// A requirement is met by what the author wrote. A value that
	// failed typing is reported once, above, under its typing code,
	// and never again as omitted.
	for i, spec := range schema.Positional {
		if spec.Required && i >= positional {
			v.report(MissingParam, raw.Pos,
				"%s omits %s, which its schema requires", d.Name, spec.Key)
			ok = false
		}
	}
	for _, spec := range schema.Params {
		_, given := seen[spec.Key]
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
					"%s param %s names %q, which no metadata key or group returns. Keys: %s",
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

// comparePos orders two raw instances by their carrier positions.
func comparePos(a, b Raw) int { return a.Pos.Compare(b.Pos) }

// rawSpelling returns a raw value's spelling for a refusal.
func rawSpelling(v RawValue) string {
	if v.List == nil {
		return v.Text
	}
	return "a list"
}
