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
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// annEntry is one scheduled annotator. The bucket number is the
// role's position in the annotate schedule, and every claim the
// annotator stamps records the same number as its arbitration
// rank's bucket.
type annEntry struct {
	bucket int
	name   plugin.ID
	run    plugin.Annotator
}

// genEntry is one scheduled generator, numbered across the whole
// generator role so one plugin serving two plans has one bucket.
type genEntry struct {
	bucket int
	name   plugin.ID
	run    plugin.Generator
}

// compiledPlan is one write side as the run executes it: the
// roles in bucket order, the sources each run binds its scope from,
// the plans it depends on, and the name that keys its store in the
// report.
type compiledPlan struct {
	name    string
	sources Sources
	// deps are the indexes of the plans this one depends on, in the
	// order its DependsOn lists them.
	deps []int
	// exported reports that a dependent plan or a workspace check reads
	// the plan's export, so a run builds it.
	exported bool
	entries  []genEntry
	backend  plugin.Backend
	// routing is the plan's validated layout, and outputs the
	// families each of its generators declares.
	routing layout.Config
	outputs map[plugin.ID][]plugin.Output
	// contract stamps the plan's rendered files, nil for a
	// composition declaring no output.
	contract *output.Contract
}

// compiledCheck is one workspace check as Close runs it.
type compiledCheck struct {
	name plugin.ID
	run  plugin.WorkspaceCheck
	// reads are the indexes of the plans the check reads, in
	// composition order: every plan of the composition for a check
	// whose Reads returns nil.
	reads []int
}

// kernelPhases lists the origins the kernel reports under. A
// plugin returning one of them would file its findings under the
// kernel's identity, so the roster refuses the name.
var kernelPhases = map[plugin.ID]bool{
	diag.PhaseBuild:    true,
	diag.PhaseLoad:     true,
	diag.PhaseLink:     true,
	diag.PhaseFreeze:   true,
	diag.PhaseAnnotate: true,
	diag.PhaseGenerate: true,
	diag.PhaseLayout:   true,
	diag.PhaseRender:   true,
	diag.PhaseClose:    true,
}

// frontendFaults is the frontend step: every registered frontend is
// present, declares a version, has a name no other frontend has, and
// is not named after a kernel phase. A frontend may share its name
// with the backend of its language, because both report under the
// language's name.
func frontendFaults(fs []plugin.Frontend) []error {
	var faults []error
	named := map[plugin.ID]bool{}
	for i, f := range fs {
		if f == nil {
			faults = append(faults, fmt.Errorf("workspace: frontend %d of %d is nil", i+1, len(fs)))
			continue
		}
		name := f.Name()
		if _, versioned := f.(plugin.Versioned); !versioned {
			faults = append(faults, fmt.Errorf(
				"workspace: frontend %q declares no version, which every unit key folds", name,
			))
		}
		switch {
		case name == "":
			faults = append(faults, errors.New("workspace: a frontend returns an empty name"))
		case named[name]:
			faults = append(faults, fmt.Errorf("workspace: two frontends return the name %q", name))
		case kernelPhases[name]:
			faults = append(faults, fmt.Errorf(
				"workspace: frontend %q is named after a kernel phase, whose findings it would report under", name,
			))
		}
		named[name] = true
	}
	return faults
}

// contractsOf returns the registered keys that promise completeness,
// in registration order: what a run's audit checks.
func contractsOf(keys *meta.Registry) []contract {
	var out []contract
	for name := range keys.Keys() {
		id, _ := keys.Resolve(name)
		spec, _ := keys.Spec(id)
		if spec.Contract != nil {
			out = append(out, contract{key: id, name: name, Completeness: *spec.Contract})
		}
	}
	return out
}

// assemble is the first step: the plugin universe, deduplicated by
// name, one name one plugin. The roster's order is the declaration
// order, annotators first, then each plan's generators and backend,
// then the checks, and it is the registration order every later step
// depends on for determinism.
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
					"workspace: two plugins return the name %q", name,
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
				continue // the plan step reports it, naming the plan
			}
			admit(g)
		}
		if pl.Backend != nil {
			admit(pl.Backend)
		}
	}
	for i, c := range b.checks {
		if c == nil {
			faults = append(faults, fmt.Errorf(
				"workspace: check %d of %d is nil", i+1, len(b.checks),
			))
			continue
		}
		admit(c)
	}
	return roster, byName, faults
}

// sameProvider reports whether a plugin arriving under a name
// already admitted is that same provider listed again: the same
// pointer, or an equal value of a comparable type. Composition
// reads a plugin through its name and never hashes the value, so a
// provider whose type is not comparable still composes. Two of them
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
// both are registries in everything but shape.
//
// Every registration binds to its registrant. A plugin registers its
// keys through a handle bound to its name, so it claims namespaces
// for itself and registers only into its own. The builder's
// registrations bind to the composition. A plugin's schema names the
// plugin that provides it, and a schema naming another plugin is
// refused. The kernel's own keys and schemas register first, so an
// impersonation is a plain duplicate by the time it arrives. The
// ignores register last, so one covering a registered name is
// refused naming it.
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
			if err := kp.Keys(keys.For(string(p.Name()))); err != nil {
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
				if s.Plugin != string(p.Name()) {
					faults = append(faults, fmt.Errorf(
						"workspace: plugin %s declares directive %s under plugin %q, "+
							"and a plugin declares schemas under its own name",
						p.Name(), s.Canonical(), s.Plugin,
					))
					continue
				}
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
// because the label alone does not identify the plugin at fault.
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
				"workspace: capability %q is provided by %s, and a label has one provider",
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
			continue // membersOf admitted it, so the plugin implements the role
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
			continue // membersOf admitted it, so the plugin implements the role
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
// naming the members left in the cycle, and those members append in
// name order, so the schedule remains total for the steps after
// this one, which run even on a faulted composition.
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
	var cycled []string
	for i, d := range indegree {
		if d > 0 {
			cycled = append(cycled, string(group[i].name))
		}
	}
	slices.Sort(cycled)
	for _, name := range cycled {
		for _, m := range group {
			if string(m.name) == name {
				out = append(out, m)
				break
			}
		}
	}
	return out, []error{fmt.Errorf(
		"workspace: capabilities cycle among %s in the %s role",
		strings.Join(cycled, " and "), role,
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
				"workspace: the config names plugin %q, which the composition does not contain",
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
			"workspace: plugin %q declares no options, and the config has a section for it",
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
			given := "nothing"
			if v != nil {
				given = reflect.TypeOf(v).String()
			}
			faults = append(faults, fmt.Errorf(
				"workspace: option %q of plugin %q takes %s, and the config has %s",
				key, p.Name(), f.Type, given,
			))
			continue
		}
		rv.FieldByIndex(f.Index).Set(reflect.ValueOf(v))
	}
	return faults
}

// compilePlans is the fifth and sixth step: every plan named once,
// at least one generator, exactly one backend against a registered
// target, every generator that declares templates serving that
// target, a layout the plan's generators' families admit, sources in
// the languages langs contains, dependencies on declared plans, and
// the roles fixed in bucket order, which is the schedule the run
// executes as data. A plan that another plan depends on is marked
// exported.
func compilePlans(
	declared []Plan, gens []genEntry, targets map[plugin.Target]bool, langs map[symbol.Lang]bool,
) ([]compiledPlan, []error) {
	var faults []error
	seatOf := map[plugin.ID]genEntry{}
	for _, s := range gens {
		seatOf[s.name] = s
	}
	at := make(map[string]int, len(declared))
	for i, pl := range declared {
		if _, taken := at[pl.Name]; !taken && pl.Name != "" {
			at[pl.Name] = i
		}
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
		outputs := map[plugin.ID][]plugin.Output{}
		var roles []genEntry
		for _, g := range pl.Generators {
			if g == nil {
				faults = append(faults, fmt.Errorf(
					"workspace: plan %q lists a nil generator", pl.Name,
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
			outputs[name] = nil
			if op, declares := g.(plugin.OutputProvider); declares {
				outputs[name] = op.Outputs()
			}
		}
		if len(roles) == 0 {
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q lists no generator", pl.Name,
			))
		}
		slices.SortFunc(roles, func(a, b genEntry) int {
			return cmp.Compare(a.bucket, b.bucket)
		})
		switch {
		case pl.Backend == nil:
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q declares no backend", pl.Name,
			))
		case !targets[pl.Backend.Target()]:
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q names target %q, which the composition does not declare",
				pl.Name, pl.Backend.Target(),
			))
		}
		faults = append(faults, unserved(pl)...)
		faults = append(faults, pl.Layout.Check(pl.Name, outputs)...)
		faults = append(faults, pl.Sources.check(pl.Name, langs)...)
		deps, dfaults := dependencies(pl, at)
		faults = append(faults, dfaults...)
		out = append(out, compiledPlan{
			name: pl.Name, sources: pl.Sources, deps: deps, entries: roles,
			backend: pl.Backend, routing: pl.Layout, outputs: outputs,
		})
	}
	for i := range out {
		for _, d := range out[i].deps {
			out[d].exported = true
		}
	}
	return out, faults
}

// dependencies resolves one plan's DependsOn into the indexes of the
// plans it names, at mapping each declared name to its first plan. It
// refuses a name listed twice, a name the composition does not
// declare, and the plan's own name, each fault naming the plan.
func dependencies(pl Plan, at map[string]int) ([]int, []error) {
	var deps []int
	var faults []error
	listed := make(map[string]bool, len(pl.DependsOn))
	for _, name := range pl.DependsOn {
		i, declared := at[name]
		switch {
		case listed[name]:
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q lists %q twice in its dependencies", pl.Name, name,
			))
		case !declared:
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q depends on %q, which the composition does not declare", pl.Name, name,
			))
		case name == pl.Name:
			faults = append(faults, fmt.Errorf("workspace: plan %q depends on itself", pl.Name))
		default:
			deps = append(deps, i)
		}
		listed[name] = true
	}
	return deps, faults
}

// orderPlans returns the plans' commit order: every plan after the
// plans it depends on, and composition order between plans that do not
// depend on each other. A cycle is one fault naming every plan in it,
// in name order. The plans the order cannot place, those of a cycle and
// those that depend on one, follow the others in composition order, so
// the order remains total for the steps that run on a faulted
// composition. Placing the plans costs O(n²) in the number of plans,
// and naming a cycle's plans costs one walk of the dependencies from
// each plan left unplaced.
func orderPlans(plans []compiledPlan) ([]int, []error) {
	waiting := make([]int, len(plans))
	dependents := make([][]int, len(plans))
	for i := range plans {
		for _, d := range plans[i].deps {
			waiting[i]++
			dependents[d] = append(dependents[d], i)
		}
	}
	order := make([]int, 0, len(plans))
	placed := make([]bool, len(plans))
	for len(order) < len(plans) {
		next := -1
		for i := range plans {
			if !placed[i] && waiting[i] == 0 {
				next = i
				break
			}
		}
		if next < 0 {
			break
		}
		placed[next] = true
		order = append(order, next)
		for _, r := range dependents[next] {
			waiting[r]--
		}
	}
	if len(order) == len(plans) {
		return order, nil
	}
	var faults []error
	reach := make([][]bool, len(plans))
	for i := range plans {
		if !placed[i] {
			reach[i] = reachable(plans, i)
		}
	}
	named := make([]bool, len(plans))
	for i := range plans {
		if placed[i] || named[i] {
			continue
		}
		var cycle []string
		for j := range plans {
			if !placed[j] && reach[i][j] && reach[j][i] {
				cycle = append(cycle, strconv.Quote(plans[j].name))
				named[j] = true
			}
		}
		if len(cycle) > 1 {
			slices.Sort(cycle)
			faults = append(faults, fmt.Errorf(
				"workspace: plans %s depend on each other in a cycle", strings.Join(cycle, " and "),
			))
		}
	}
	for i := range plans {
		if !placed[i] {
			order = append(order, i)
		}
	}
	return order, faults
}

// reachable returns, for each plan, whether a walk of the dependencies
// that starts at from and takes at least one step arrives at it.
func reachable(plans []compiledPlan, from int) []bool {
	seen := make([]bool, len(plans))
	stack := slices.Clone(plans[from].deps)
	for len(stack) > 0 {
		at := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[at] {
			continue
		}
		seen[at] = true
		stack = append(stack, plans[at].deps...)
	}
	return seen
}

// compileChecks resolves each check's reads into the indexes of the
// plans it names, sorted into composition order, and marks every plan
// a check reads as exported. A check whose Reads returns nil reads
// every plan. It refuses a plan read twice and a plan the composition
// does not declare, each fault naming the check. A nil check is the
// roster's fault, and it is skipped here.
func compileChecks(declared []plugin.WorkspaceCheck, plans []compiledPlan) ([]compiledCheck, []error) {
	at := make(map[string]int, len(plans))
	for i := range plans {
		if _, taken := at[plans[i].name]; !taken {
			at[plans[i].name] = i
		}
	}
	var faults []error
	out := make([]compiledCheck, 0, len(declared))
	for _, c := range declared {
		if c == nil {
			continue
		}
		name, names := c.Name(), c.Reads()
		var reads []int
		if names == nil {
			reads = make([]int, 0, len(plans))
			for i := range plans {
				reads = append(reads, i)
			}
		}
		listed := make(map[string]bool, len(names))
		for _, plan := range names {
			i, declared := at[plan]
			switch {
			case listed[plan]:
				faults = append(faults, fmt.Errorf("workspace: check %s reads plan %q twice", name, plan))
			case !declared:
				faults = append(faults, fmt.Errorf(
					"workspace: check %s reads plan %q, which the composition does not declare", name, plan,
				))
			default:
				reads = append(reads, i)
			}
			listed[plan] = true
		}
		slices.Sort(reads)
		for _, i := range reads {
			plans[i].exported = true
		}
		out = append(out, compiledCheck{name: name, run: c, reads: reads})
	}
	return out, faults
}

// languages returns the languages the composition knows: each
// registered frontend's and each registered rules value's. A plan's
// sources name one of them.
func languages(fs []plugin.Frontend, rs *rules.Registry) map[symbol.Lang]bool {
	langs := map[symbol.Lang]bool{}
	for _, f := range fs {
		if f != nil {
			langs[f.Lang()] = true
		}
	}
	for _, lang := range rs.Languages() {
		langs[lang] = true
	}
	return langs
}

// unserved refuses each generator of a plan that declares template
// trees of its own and none that serves the plan's target: every
// reference it emits would resolve to nothing at render, so the
// composition refuses it at Build, naming the plugin and the
// targets it declares. A generator with a tree for every target,
// or with no tree at all, serves any target.
func unserved(pl Plan) []error {
	if pl.Backend == nil {
		return nil // the plan's own fault is already collected
	}
	target := pl.Backend.Target()
	var faults []error
	for _, g := range pl.Generators {
		tp, presents := g.(plugin.TemplateProvider)
		if !presents {
			continue
		}
		if _, served := tp.Templates(target); served {
			continue
		}
		declared := tp.TemplateTargets()
		if len(declared) == 0 {
			continue
		}
		spelled := make([]string, len(declared))
		for i, t := range declared {
			spelled[i] = strconv.Quote(string(t))
		}
		faults = append(faults, fmt.Errorf(
			"workspace: plan %q targets %q, and %s declares templates for %s alone",
			pl.Name, target, g.Name(), strings.Join(spelled, " and "),
		))
	}
	return faults
}

// stampable builds each plan's output contract from the brand and
// the backend's own comment syntax, and reports what cannot be
// written: a backend that does not render, a backend that spells no
// filenames, an invalid brand, or a backend stating no syntax to
// frame a generated file through. It runs only where a composition
// declares output, because a run that writes nothing needs no
// layout, no render and no frame.
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
		if _, spells := pl.backend.(plugin.FileSpeller); !spells {
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q writes output and its backend %s spells no filenames",
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
