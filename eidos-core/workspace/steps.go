// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// annEntry is one scheduled annotator. The bucket number is the
// role's position in the annotate schedule, and the same number
// every claim it stamps carries as the arbitration rank's bucket.
type annEntry struct {
	bucket int
	name   plugin.ID
	run    plugin.Annotator
}

// genEntry is one scheduled generator, numbered across the whole
// generator role so one plugin serving two plans holds one bucket.
type genEntry struct {
	bucket int
	name   plugin.ID
	run    plugin.Generator
}

// compiledPlan is one write side as the run executes it: the
// roles in bucket order, the scope, and the name that keys its
// store in the report.
type compiledPlan struct {
	name    string
	scope   store.Scope
	entries []genEntry
	backend plugin.Backend
	layout  func(pkg symbol.Identity, name string) string
	// contract stamps the plan's rendered files, nil for a
	// composition declaring no output.
	contract *output.Contract
}

// kernelPhases holds the origins the kernel reports under. A
// plugin returning one of them would file its findings under the
// kernel's identity, so the roster refuses the name.
var kernelPhases = map[plugin.ID]bool{
	diag.PhaseBuild:    true,
	diag.PhaseLoad:     true,
	diag.PhaseLink:     true,
	diag.PhaseFreeze:   true,
	diag.PhaseAnnotate: true,
	diag.PhaseGenerate: true,
	diag.PhaseRender:   true,
	diag.PhaseClose:    true,
}

// assemble is the first step: the plugin universe, deduplicated by
// name, one name one plugin. The roster's order is the declaration
// order, annotators first, then each plan's generators and backend,
// and it is the registration order every later step leans on for
// determinism.
func (b *Builder) assemble() ([]plugin.Plugin, map[plugin.ID]plugin.Plugin, []error) {
	var roster []plugin.Plugin
	var faults []error
	byName := map[plugin.ID]plugin.Plugin{}
	admit := func(p plugin.Plugin) {
		name := p.Name()
		if name == "" {
			faults = append(faults,
				errors.New("workspace: a plugin returns an empty name"))
			return
		}
		if held, seated := byName[name]; seated {
			if !sameProvider(held, p) {
				faults = append(faults, fmt.Errorf(
					"workspace: two plugins carry the name %q", name,
				))
			}
			return
		}
		if kernelPhases[name] {
			faults = append(faults, fmt.Errorf(
				"workspace: plugin %q is named after a kernel phase, whose findings it would report under",
				name,
			))
		}
		byName[name] = p
		roster = append(roster, p)
	}
	for i, a := range b.annotators {
		if a == nil {
			faults = append(faults, fmt.Errorf(
				"workspace: annotator %d of %d is nil", i+1, len(b.annotators),
			))
			continue
		}
		admit(a)
	}
	for _, pl := range b.plans {
		for _, g := range pl.Generators {
			if g == nil {
				continue // the plan step bills it, naming the plan
			}
			admit(g)
		}
		if pl.Backend != nil {
			admit(pl.Backend)
		}
	}
	return roster, byName, faults
}

// sameProvider reports whether a plugin arriving under a name
// already seated is that same provider listed again: the same
// pointer, or an equal value of a comparable type. Composition
// reads a plugin through its name and never hashes the value, so a
// provider whose type is no map key still composes; two of them
// under one name are two plugins, because nothing tells one copy
// from another.
func sameProvider(seated, p plugin.Plugin) bool {
	v := reflect.ValueOf(seated)
	return v.Comparable() && v.Equal(reflect.ValueOf(p))
}

// register is the second step: the kernel's directive schemas
// first, because validation of the skip and meta instances reads
// them, then every plugin's keys and the builder's own
// registrations, the key registry's seal, every plugin's schemas,
// and the directive seal that resolves constraints.
// Capability labels and target names collect here too, because
// both are registries in everything but shape. The kernel's own
// keys and schemas register first, so an impersonation is a plain
// duplicate by the time it arrives. The ignores register last, so
// one covering a registered name is refused naming it.
func (b *Builder) register(roster []plugin.Plugin) (registries, []error) {
	var faults []error
	keys := meta.NewRegistry()
	kernel, err := meta.Kernel(keys)
	if err != nil {
		faults = append(faults, err)
	}
	dirs := directive.NewRegistry()
	for _, s := range directive.Kernel() {
		if err := dirs.Register(s); err != nil {
			faults = append(faults, err)
		}
	}
	for _, p := range roster {
		if kp, held := p.(plugin.KeyProvider); held {
			if err := kp.Keys(keys); err != nil {
				faults = append(faults, err)
			}
		}
	}
	for i, register := range b.keys {
		if register == nil {
			faults = append(faults, fmt.Errorf(
				"workspace: key registration %d of %d is nil", i+1, len(b.keys),
			))
			continue
		}
		if err := register(keys); err != nil {
			faults = append(faults, err)
		}
	}
	keys.Seal()
	for _, p := range roster {
		if dp, held := p.(plugin.DirectiveProvider); held {
			for _, s := range dp.Directives() {
				if err := dirs.Register(s); err != nil {
					faults = append(faults, err)
				}
			}
		}
	}
	for _, n := range b.ignored {
		if err := dirs.Ignore(n); err != nil {
			faults = append(faults, err)
		}
	}
	faults = append(faults, dirs.Seal()...)
	faults = append(faults, capabilities(roster)...)

	targets := map[plugin.Target]bool{}
	for _, t := range b.targets {
		switch {
		case t == "":
			faults = append(faults,
				errors.New("workspace: a target name is empty"))
		case targets[t]:
			faults = append(faults, fmt.Errorf(
				"workspace: target %q is declared twice", t,
			))
		default:
			targets[t] = true
		}
	}
	langs := rules.NewRegistry()
	for _, r := range b.rules {
		if r == nil {
			faults = append(faults, errors.New("workspace: a registered rules value is nil"))
			continue
		}
		if err := langs.Register(r); err != nil {
			faults = append(faults, err)
		}
	}
	return registries{keys: keys, kernel: kernel, directives: dirs, rules: langs, targets: targets}, faults
}

// registries is what the register step hands the workspace: every
// sealed registry and the kernel's own keys.
type registries struct {
	keys       *meta.Registry
	kernel     meta.KernelKeys
	directives *directive.Registry
	rules      *rules.Registry
	targets    map[plugin.Target]bool
}

// capabilities collects the labels: one provider per label, and a
// provider for every requirement. Both refusals name the plugins,
// because the label itself cannot say who is wrong.
func capabilities(roster []plugin.Plugin) []error {
	var faults []error
	providers := map[plugin.Capability][]plugin.ID{}
	for _, p := range roster {
		cp, held := p.(plugin.CapabilityProvider)
		if !held {
			continue
		}
		mine := map[plugin.Capability]bool{}
		for _, c := range cp.Provides() {
			if c == "" {
				faults = append(faults, fmt.Errorf(
					"workspace: %s provides an empty capability label", p.Name(),
				))
				continue
			}
			if mine[c] {
				continue
			}
			mine[c] = true
			providers[c] = append(providers[c], p.Name())
		}
	}
	for _, c := range slices.Sorted(maps.Keys(providers)) {
		if names := providers[c]; len(names) > 1 {
			spelled := make([]string, len(names))
			for i, n := range names {
				spelled[i] = string(n)
			}
			faults = append(faults, fmt.Errorf(
				"workspace: capability %q is provided by %s, and a label holds one provider",
				c, strings.Join(spelled, " and "),
			))
		}
	}
	for _, p := range roster {
		cp, held := p.(plugin.CapabilityProvider)
		if !held {
			continue
		}
		asked := map[plugin.Capability]bool{}
		for _, c := range cp.Requires() {
			if c == "" {
				faults = append(faults, fmt.Errorf(
					"workspace: %s requires an empty capability label", p.Name(),
				))
				continue
			}
			if asked[c] {
				continue
			}
			asked[c] = true
			if len(providers[c]) == 0 {
				faults = append(faults, fmt.Errorf(
					"workspace: %s requires capability %q, which nothing provides",
					p.Name(), c,
				))
			}
		}
	}
	return faults
}

// member is one role candidate with its ordering inputs read off
// the provider surfaces as data.
type member struct {
	p        plugin.Plugin
	name     plugin.ID
	pri      int
	provides []plugin.Capability
	requires []plugin.Capability
}

// membersOf reads one role's candidates off the roster, in roster
// order.
func membersOf(roster []plugin.Plugin, role plugin.Role, holds func(plugin.Plugin) bool) []member {
	var out []member
	for _, p := range roster {
		if !holds(p) {
			continue
		}
		m := member{p: p, name: p.Name()}
		if cp, held := p.(plugin.CapabilityProvider); held {
			m.pri = cp.Priority(role)
			m.provides = cp.Provides()
			m.requires = cp.Requires()
		}
		out = append(out, m)
	}
	return out
}

// lower is the third step: each role's members sort by priority,
// then by capability topology inside one priority, then by name,
// and a member's bucket number is its position in the result.
func lower(roster []plugin.Plugin) ([]annEntry, []genEntry, []error) {
	var faults []error

	annotators, aerr := order("annotator", membersOf(roster, plugin.RoleAnnotator,
		func(p plugin.Plugin) bool { _, held := p.(plugin.Annotator); return held }))
	faults = append(faults, aerr...)
	ann := make([]annEntry, 0, len(annotators))
	for i, m := range annotators {
		run, held := m.p.(plugin.Annotator)
		if !held {
			continue // membersOf admitted it, so the role holds
		}
		ann = append(ann, annEntry{bucket: i + 1, name: m.name, run: run})
	}

	generators, gerr := order("generator", membersOf(roster, plugin.RoleGenerator,
		func(p plugin.Plugin) bool { _, held := p.(plugin.Generator); return held }))
	faults = append(faults, gerr...)
	gen := make([]genEntry, 0, len(generators))
	for i, m := range generators {
		run, held := m.p.(plugin.Generator)
		if !held {
			continue // membersOf admitted it, so the role holds
		}
		gen = append(gen, genEntry{bucket: i + 1, name: m.name, run: run})
	}
	return ann, gen, faults
}

// order sorts one role's members into schedule order: priority
// ascending, capability topology inside one priority, names
// breaking what remains open.
func order(role string, ms []member) ([]member, []error) {
	slices.SortFunc(ms, func(a, b member) int {
		if c := cmp.Compare(a.pri, b.pri); c != 0 {
			return c
		}
		return cmp.Compare(a.name, b.name)
	})
	var out []member
	var faults []error
	for start := 0; start < len(ms); {
		end := start
		for end < len(ms) && ms[end].pri == ms[start].pri {
			end++
		}
		group, ferr := topo(role, ms[start:end])
		out = append(out, group...)
		faults = append(faults, ferr...)
		start = end
	}
	return out, faults
}

// topo orders one priority group by its capability edges, provider
// before requirer, smallest ready name first. A cycle is one fault
// naming the members still standing, and those members append in
// name order so the schedule stays total for the steps after this
// one, which run even on a faulted composition.
func topo(role string, group []member) ([]member, []error) {
	if len(group) < 2 {
		return group, nil
	}
	providers := map[plugin.Capability][]int{}
	for i, m := range group {
		for _, c := range m.provides {
			providers[c] = append(providers[c], i)
		}
	}
	indegree := make([]int, len(group))
	after := make([][]int, len(group))
	for i, m := range group {
		for _, c := range m.requires {
			for _, p := range providers[c] {
				after[p] = append(after[p], i)
				indegree[i]++
			}
		}
	}
	var ready []int
	for i, d := range indegree {
		if d == 0 {
			ready = append(ready, i)
		}
	}
	out := make([]member, 0, len(group))
	for len(ready) > 0 {
		slices.SortFunc(ready, func(a, b int) int {
			return cmp.Compare(group[a].name, group[b].name)
		})
		i := ready[0]
		ready = ready[1:]
		out = append(out, group[i])
		for _, r := range after[i] {
			if indegree[r]--; indegree[r] == 0 {
				ready = append(ready, r)
			}
		}
	}
	if len(out) == len(group) {
		return out, nil
	}
	var standing []string
	for i, d := range indegree {
		if d > 0 {
			standing = append(standing, string(group[i].name))
		}
	}
	slices.Sort(standing)
	for _, name := range standing {
		for _, m := range group {
			if string(m.name) == name {
				out = append(out, m)
				break
			}
		}
	}
	return out, []error{fmt.Errorf(
		"workspace: capabilities cycle among %s in the %s role",
		strings.Join(standing, " and "), role,
	)}
}

// configure is the fourth step: every plugin's options validate
// against the tag contract, then the config's values populate the
// structs over their constructed defaults, and each populated
// struct is taken in the canonical encoding the fingerprint folds.
// A plugin whose schema failed skips population and encoding,
// because its faults are already collected. An options struct the
// encoding cannot see whole is a fault, because the fingerprint
// folds every option.
func configure(
	roster []plugin.Plugin, byName map[plugin.ID]plugin.Plugin, cfg Config,
) (map[plugin.ID][]byte, []error) {
	var faults []error
	sound := map[plugin.ID]bool{}
	for _, p := range roster {
		errs := plugin.ValidateOptions(p)
		faults = append(faults, errs...)
		sound[p.Name()] = len(errs) == 0
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Options)) {
		p, held := byName[plugin.ID(name)]
		if !held {
			faults = append(faults, fmt.Errorf(
				"workspace: the config names plugin %q, which the composition does not hold",
				name,
			))
			continue
		}
		if !sound[plugin.ID(name)] {
			continue
		}
		faults = append(faults, populate(p, cfg.Options[name])...)
	}
	encodings := make(map[plugin.ID][]byte, len(roster))
	for _, p := range roster {
		if !sound[p.Name()] {
			continue
		}
		encoded, err := plugin.EncodeOptions(p)
		if err != nil {
			faults = append(faults, fmt.Errorf("workspace: %w", err))
			continue
		}
		encodings[p.Name()] = encoded
	}
	return encodings, faults
}

// populate sets one plugin's declared options from its config
// section, key by key, sorted so the faults arrive in one order.
func populate(p plugin.Plugin, section map[string]any) []error {
	op, held := p.(plugin.OptionsProvider)
	if !held || op.Options() == nil {
		return []error{fmt.Errorf(
			"workspace: plugin %q declares no options, and the config carries a section for it",
			p.Name(),
		)}
	}
	rv := reflect.ValueOf(op.Options()).Elem()
	rt := rv.Type()
	byKey := map[string]reflect.StructField{}
	for f := range rt.Fields() {
		byKey[f.Tag.Get("opt")] = f
	}
	var faults []error
	for _, key := range slices.Sorted(maps.Keys(section)) {
		f, declared := byKey[key]
		if !declared {
			faults = append(faults, fmt.Errorf(
				"workspace: plugin %q declares no option %q", p.Name(), key,
			))
			continue
		}
		v := section[key]
		if v == nil || !reflect.TypeOf(v).AssignableTo(f.Type) {
			carried := "nothing"
			if v != nil {
				carried = reflect.TypeOf(v).String()
			}
			faults = append(faults, fmt.Errorf(
				"workspace: option %q of plugin %q wants %s, the config carries %s",
				key, p.Name(), f.Type, carried,
			))
			continue
		}
		rv.FieldByIndex(f.Index).Set(reflect.ValueOf(v))
	}
	return faults
}

// compilePlans is the fifth and sixth step: every plan named once,
// at least one generator, exactly one backend against a registered
// target, and the roles fixed in bucket order, which is the
// schedule the run executes as data.
func compilePlans(
	declared []Plan, gens []genEntry, targets map[plugin.Target]bool,
) ([]compiledPlan, []error) {
	var faults []error
	seatOf := map[plugin.ID]genEntry{}
	for _, s := range gens {
		seatOf[s.name] = s
	}
	names := map[string]bool{}
	out := make([]compiledPlan, 0, len(declared))
	for _, pl := range declared {
		switch {
		case pl.Name == "":
			faults = append(faults,
				errors.New("workspace: a plan has no name"))
		case names[pl.Name]:
			faults = append(faults, fmt.Errorf(
				"workspace: the plan name %q is declared twice", pl.Name,
			))
		default:
			names[pl.Name] = true
		}
		listed := map[plugin.ID]bool{}
		var roles []genEntry
		for _, g := range pl.Generators {
			if g == nil {
				faults = append(faults, fmt.Errorf(
					"workspace: plan %q holds a nil generator", pl.Name,
				))
				continue
			}
			name := g.Name()
			if listed[name] {
				faults = append(faults, fmt.Errorf(
					"workspace: plan %q lists %s twice", pl.Name, name,
				))
				continue
			}
			listed[name] = true
			roles = append(roles, seatOf[name])
		}
		if len(roles) == 0 {
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q holds no generator", pl.Name,
			))
		}
		slices.SortFunc(roles, func(a, b genEntry) int {
			return cmp.Compare(a.bucket, b.bucket)
		})
		switch {
		case pl.Backend == nil:
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q holds no backend", pl.Name,
			))
		case !targets[pl.Backend.Target()]:
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q names target %q, which the composition does not declare",
				pl.Name, pl.Backend.Target(),
			))
		}
		out = append(out, compiledPlan{
			name: pl.Name, scope: pl.Scope, entries: roles,
			backend: pl.Backend, layout: pl.Layout,
		})
	}
	return out, faults
}

// stampable builds each plan's output contract from the brand and
// the backend's own comment syntax, and reports what cannot be
// written: a backend that does not render, an invalid brand, or a
// backend stating no syntax to frame a generated file through. It
// runs only where a composition declares output, because a run
// that writes nothing needs no render and no frame.
func stampable(plans []compiledPlan, brand output.Brand) []error {
	var faults []error
	for i := range plans {
		pl := &plans[i]
		if pl.backend == nil {
			continue // the plan's own fault is already collected
		}
		if _, renders := pl.backend.(plugin.Renderer); !renders {
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q writes output and its backend %s does not render",
				pl.name, pl.backend.Name(),
			))
		}
		syn, states := pl.backend.(plugin.SyntaxProvider)
		if !states {
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q writes output and its backend %s states no "+
					"comment syntax to frame a generated file through",
				pl.name, pl.backend.Name(),
			))
			continue
		}
		contract, err := output.NewContract(brand, syn.Syntax())
		if err != nil {
			faults = append(faults, fmt.Errorf("workspace: plan %q: %w", pl.name, err))
			continue
		}
		pl.contract = contract
	}
	return faults
}
