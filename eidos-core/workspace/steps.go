// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// annEntry is one scheduled annotator. Its turn is the position of
// the role in the annotate schedule, and every claim of the annotator
// records the same turn in its rank.
type annEntry struct {
	turn int
	name plugin.ID
	run  plugin.Annotator
}

// genEntry is one scheduled generator. The turns count across the
// whole generator role, so one plugin that serves two plans has one
// turn.
type genEntry struct {
	turn int
	name plugin.ID
	run  plugin.Generator
}

// compiledPlan is one write side as the run executes it: the
// roles in turn order, the sources each run binds its scope from,
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
	// backend is the backend of the plan, which each generator receives
	// as the spoke of the plan, and policy the plan's resolved lowering
	// policy, which each generator receives too.
	backend plugin.Backend
	policy  plugin.Policy
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
// present, has a name no other frontend has, and is not named after a
// kernel phase. A frontend may share its name with the backend of its
// language, because both report under the language's name.
func frontendFaults(fs []plugin.Frontend) []error {
	var faults []error
	named := map[plugin.ID]bool{}
	for i, f := range fs {
		if f == nil {
			faults = append(faults, fmt.Errorf("workspace: frontend %d of %d is nil", i+1, len(fs)))
			continue
		}
		name := f.Name()
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

// frontendOptions is the frontends' share of the configure step: each
// frontend's options in the canonical encoding that every unit key and
// the fingerprint fold, in load order, nil for a frontend that declares
// none and for a nil frontend, which the frontend step reports. A
// frontend whose options the encoding cannot see whole is a fault.
func frontendOptions(fs []plugin.Frontend) ([][]byte, []error) {
	var faults []error
	out := make([][]byte, len(fs))
	for i, f := range fs {
		if f == nil {
			continue
		}
		encoded, err := plugin.EncodeOptions(f)
		if err != nil {
			faults = append(faults, fmt.Errorf("workspace: frontend %q: %w", f.Name(), err))
			continue
		}
		out[i] = encoded
	}
	return out, faults
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
// order, annotators first, then for each plan its generators, the
// generators that the config adds to it and its backend, then the
// checks. Every later step registers in this order, so the order makes
// the composition deterministic. The roster contains the generators
// that the config adds to a plan that refine refuses, so the configure
// step populates their options and reports no second error about them.
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
		for _, gens := range [...][]plugin.Generator{pl.Generators, b.config.Plans[pl.Name].Generators} {
			for _, g := range gens {
				if g == nil {
					continue // the plan step reports it, naming the plan
				}
				admit(g)
			}
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

// register is the register step. The kernel's keys and directive
// schemas register first, because validation of the skip and meta
// instances reads the schemas. The keys of each frontend follow in load
// order, then the keys of each lowering entry's target, then every
// plugin's keys in roster order and the builder's own registrations,
// and then the key registry's seal. Each target's directive registers
// after the kernel's schemas, and every plugin's schemas and the
// directive seal that resolves constraints follow it. Capability labels
// and target names collect here too, because both are registries in
// everything but shape.
//
// Every registration binds to its registrant. A frontend registers its
// language's keys through a handle bound to the language's spelling.
// The workspace registers a target's keys through a handle bound to the
// target's spelling, so the parts of one language are one registrant. A
// plugin registers its keys through a handle bound to its name, so it
// claims namespaces for itself and registers only into its own. The
// builder's registrations bind to the composition. The schema of a
// plugin has the plugin that provides it, and a schema of another plugin
// is refused. The kernel's own keys and schemas register first, so an
// impersonation is a plain duplicate by the time it arrives. The ignores
// register last, so an ignore that covers a registered name is refused
// with that name.
func (b *Builder) register(roster []plugin.Plugin, entries []*lowering) (registries, []error) {
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
	for _, l := range entries {
		if err := dirs.RegisterTarget(l.schema()); err != nil {
			faults = append(faults, err)
		}
	}
	for _, f := range b.frontends {
		if kp, held := f.(plugin.KeyProvider); held {
			if err := kp.Keys(keys.For(string(f.Lang()))); err != nil {
				faults = append(faults, err)
			}
		}
	}
	for _, l := range entries {
		faults = append(faults, l.register(keys)...)
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

// providers are the provider interfaces of more than one method. The
// workspace reads such a provider only from a plugin that implements every
// method, so it refuses a plugin that implements part of one.
var providers = []reflect.Type{
	reflect.TypeFor[plugin.CapabilityProvider](),
	reflect.TypeFor[plugin.TemplateProvider](),
}

// partialProviders returns one fault for each plugin of the roster that
// implements some methods of one of the providers and lacks others. A
// method counts where the type of the plugin has a method of its name and
// its signature. The fault names the methods that the plugin implements
// and the methods that it lacks. A plugin that implements a whole provider,
// or none of its methods, costs no allocation.
func partialProviders(roster []plugin.Plugin) []error {
	var faults []error
	for _, p := range roster {
		t := reflect.TypeOf(p)
		for _, provider := range providers {
			if t.Implements(provider) {
				continue
			}
			implemented := false
			for m := range provider.Methods() {
				implemented = implemented || implements(t, m)
			}
			if !implemented {
				continue
			}
			var has, lacks []string
			for m := range provider.Methods() {
				if implements(t, m) {
					has = append(has, m.Name)
				} else {
					lacks = append(lacks, m.Name)
				}
			}
			faults = append(faults, fmt.Errorf(
				"workspace: %s implements %s of %s, and lacks %s",
				p.Name(), strings.Join(has, " and "), provider, strings.Join(lacks, " and "),
			))
		}
	}
	return faults
}

// implements reports whether the concrete type t has a method with the
// name and the signature of the interface method m. It allocates nothing
// for a type without a method of that name.
func implements(t reflect.Type, m reflect.Method) bool {
	got, held := t.MethodByName(m.Name)
	if !held || got.Type.NumIn() != m.Type.NumIn()+1 || got.Type.NumOut() != m.Type.NumOut() ||
		got.Type.IsVariadic() != m.Type.IsVariadic() {

		return false
	}
	for i := range m.Type.NumIn() {
		if got.Type.In(i+1) != m.Type.In(i) {
			return false
		}
	}
	for i := range m.Type.NumOut() {
		if got.Type.Out(i) != m.Type.Out(i) {
			return false
		}
	}
	return true
}

// capabilities collects the labels. It refuses a label with two
// providers, a required label without a provider, and an empty label.
// Each refusal names the plugins, because the label alone does not
// identify the plugin at fault. A label that a plugin runs after needs no
// provider.
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
		if slices.Contains(cp.After(), "") {
			faults = append(faults, fmt.Errorf(
				"workspace: %s runs after an empty capability label", p.Name(),
			))
		}
	}
	return faults
}

// member is one candidate of a role, with the inputs of its order that the
// workspace reads off its provider as data.
type member struct {
	p        plugin.Plugin
	name     plugin.ID
	pri      int
	provides []plugin.Capability
	requires []plugin.Capability
	after    []plugin.Capability
}

// orderings are the verbs of the two lists of capabilities whose providers
// run before a member. The first verb is the verb of the required
// capabilities, and the second the verb of the capabilities that the
// member runs after.
var orderings = [...]string{"requires", "runs after"}

// membersOf reads the candidates of one role off the roster, in roster
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
			m.after = cp.After()
		}
		out = append(out, m)
	}
	return out
}

// lower is the schedule step. The members of each role sort by priority,
// then by their capabilities inside one priority, then by name, and the
// turn of a member is its position in the result. The lowering
// entries run before every annotator, in the order of their names, so
// each annotator reads every target name. Every generator runs after the
// annotators, so lower returns a fault for an annotator that requires or
// runs after a capability that only a generator provides.
func lower(roster []plugin.Plugin, entries []*lowering) ([]annEntry, []genEntry, []error) {
	annotators, aerr := order("annotator", membersOf(roster, plugin.RoleAnnotator,
		func(p plugin.Plugin) bool { _, held := p.(plugin.Annotator); return held }))
	ann := make([]annEntry, 0, len(entries)+len(annotators))
	for _, l := range entries {
		ann = append(ann, annEntry{turn: len(ann) + 1, name: l.Name(), run: l})
	}
	for _, m := range annotators {
		run, held := m.p.(plugin.Annotator)
		if !held {
			continue // membersOf admitted it, so the plugin implements the role
		}
		ann = append(ann, annEntry{turn: len(ann) + 1, name: m.name, run: run})
	}

	generators, gerr := order("generator", membersOf(roster, plugin.RoleGenerator,
		func(p plugin.Plugin) bool { _, held := p.(plugin.Generator); return held }))
	gen := make([]genEntry, 0, len(generators))
	for i, m := range generators {
		run, held := m.p.(plugin.Generator)
		if !held {
			continue // membersOf admitted it, so the plugin implements the role
		}
		gen = append(gen, genEntry{turn: i + 1, name: m.name, run: run})
	}
	faults := slices.Concat(aerr, gerr)
	for _, a := range annotators {
		for k, caps := range [...][]plugin.Capability{a.requires, a.after} {
			for i, c := range caps {
				if slices.Contains(caps[:i], c) || k > 0 && slices.Contains(a.requires, c) ||
					slices.ContainsFunc(annotators, func(p member) bool { return slices.Contains(p.provides, c) }) {

					continue
				}
				for _, g := range generators {
					if slices.Contains(g.provides, c) {
						faults = append(faults, fmt.Errorf(
							"workspace: annotator %s %s capability %q, which the generator %s provides, "+
								"and every generator runs after the annotators", a.name, orderings[k], c, g.name,
						))
					}
				}
			}
		}
	}
	return ann, gen, faults
}

// bindKeys is the binding step. Each annotator and generator of the
// roster that implements [plugin.KeyBinder] binds its gates on named key
// handles in the sealed key registry. The bound plugin takes the place of
// the plugin in the roster and in byName, so the schedule runs the bound
// plugin. The step refuses a bound plugin of another type or another name
// than its plugin.
func bindKeys(roster []plugin.Plugin, byName map[plugin.ID]plugin.Plugin, keys *meta.Registry) []error {
	var faults []error
	for i, p := range roster {
		kb, binds := p.(plugin.KeyBinder)
		_, annotates := p.(plugin.Annotator)
		_, generates := p.(plugin.Generator)
		if !binds || !annotates && !generates {
			continue
		}
		bound, err := kb.BindKeys(keys)
		if err != nil {
			faults = append(faults, err)
		}
		if reflect.TypeOf(bound) != reflect.TypeOf(p) || bound.Name() != p.Name() {
			faults = append(faults, fmt.Errorf(
				"workspace: %s returns a plugin of another type or another name from BindKeys", p.Name(),
			))
			continue
		}
		roster[i] = bound
		byName[p.Name()] = bound
	}
	return faults
}

// gateOrder refuses each annotator that gates on a key of another
// annotator and does not run after that annotator. Such a gate must read
// the stamps of the registrant whole, so the registrant runs in an earlier
// turn. gateOrder returns one fault for each annotator and key, and the
// message of the fault contains the capabilities that order the two
// annotators. The claimant of the namespace of a key is the registrant of
// the key.
func gateOrder(ann []annEntry, keys *meta.Registry) []error {
	var faults []error
	for _, a := range ann {
		subscribed, declares := a.run.(plugin.Subscribed)
		if !declares {
			continue
		}
		refused := map[meta.KeyID]bool{}
		for _, s := range subscribed.Subscriptions() {
			spec, registered := keys.Spec(s.FactKey)
			if !registered || refused[s.FactKey] {
				continue
			}
			claimant, _ := keys.Claimant(spec.Name.Namespace())
			at := slices.IndexFunc(ann, func(e annEntry) bool { return e.name == plugin.ID(claimant) })
			if at < 0 || ann[at].name == a.name || ann[at].turn < a.turn {
				continue
			}
			registrant := ann[at]
			refused[s.FactKey] = true
			faults = append(faults, fmt.Errorf(
				"workspace: annotator %s gates on %s, which annotator %s registers, and %s does not run after %s: %s",
				a.name, spec.Name, registrant.name, a.name, registrant.name, requirable(registrant)))
		}
	}
	return faults
}

// requirable returns the text of what an annotator requires to run after
// a registrant. The text lists the capabilities of the registrant, or
// reports that the registrant provides none.
func requirable(registrant annEntry) string {
	var labels []string
	if cp, provides := registrant.run.(plugin.CapabilityProvider); provides {
		for _, c := range cp.Provides() {
			labels = append(labels, strconv.Quote(string(c)))
		}
	}
	if len(labels) == 0 {
		return fmt.Sprintf("%s provides no capability to require", registrant.name)
	}
	return "require " + strings.Join(labels, " or ")
}

// order sorts the members of one role into schedule order. It sorts them
// by priority, then by their capabilities inside one priority, and then
// by name. A member that requires or runs after a capability whose
// provider in the role has a later priority would run before its
// provider, and order returns one fault for each such pair.
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
		end := start + 1
		for end < len(ms) && ms[end].pri == ms[start].pri {
			end++
		}
		group, ferr := topo(role, ms[start:end])
		out = append(out, group...)
		faults = append(faults, ferr...)
		start = end
	}
	for _, m := range ms {
		for k, caps := range [...][]plugin.Capability{m.requires, m.after} {
			for i, c := range caps {
				if slices.Contains(caps[:i], c) || k > 0 && slices.Contains(m.requires, c) {
					continue
				}
				for _, p := range ms {
					if p.pri > m.pri && slices.Contains(p.provides, c) {
						faults = append(faults, fmt.Errorf(
							"workspace: %s %s %s capability %q at priority %d, and %s provides it at priority %d, "+
								"so %s runs after %s", role, m.name, orderings[k], c, m.pri, p.name, p.pri, p.name, m.name,
						))
					}
				}
			}
		}
	}
	return out, faults
}

// topo orders one priority group by its capabilities. A provider runs
// before each member that requires its capability or runs after it, and
// the smallest ready name runs first. A cycle is one fault that names the
// members left in the cycle. Those members follow in name order, so the
// schedule remains total for the steps after this one, which run even on
// a faulted composition.
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
		for _, caps := range [...][]plugin.Capability{m.requires, m.after} {
			for _, c := range caps {
				for _, p := range providers[c] {
					after[p] = append(after[p], i)
					indegree[i]++
				}
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

// configure is the options step. It works through the options of every
// plugin of the roster in three parts:
//
//   - [plugin.ValidateOptions] checks each options struct against the tag
//     contract.
//   - The values of the config populate the structs over their constructed
//     defaults.
//   - [plugin.MissingOptions] reports each required option that still has
//     the zero value of its type.
//
// configure then takes each struct in the canonical encoding that the
// fingerprint folds. A plugin whose declaration fails skips the
// population, the check of its required options and the encoding,
// because its faults are already collected. An options struct that the
// encoding cannot see whole is a fault, because the fingerprint folds
// every option.
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
		faults = append(faults, plugin.MissingOptions(p)...)
		encoded, err := plugin.EncodeOptions(p)
		if err != nil {
			faults = append(faults, fmt.Errorf("workspace: %w", err))
			continue
		}
		encodings[p.Name()] = encoded
	}
	return encodings, faults
}

// populate sets the declared options of one plugin from its config
// section. It sets them key by key in sorted order, so the faults arrive
// in one order. A value that the field takes as it is fills the field, and
// [convert] turns any other value into the type of the field. A key that
// the plugin does not declare, an empty value and a value that convert
// refuses are faults.
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
		if v == nil {
			faults = append(faults, fmt.Errorf(
				"workspace: option %q of plugin %q takes %s, and the config has nothing", key, p.Name(), f.Type,
			))
			continue
		}
		value := reflect.ValueOf(v)
		if !value.Type().AssignableTo(f.Type) {
			converted, err := convert(p.Name(), key, v, f.Type)
			if err != nil {
				faults = append(faults, err)
				continue
			}
			value = converted
		}
		rv.FieldByIndex(f.Index).Set(value)
	}
	return faults
}

// durationType is the type of an option that [convert] parses from a text.
var durationType = reflect.TypeFor[time.Duration]()

// convert returns the value v of the option key of the plugin name as a
// value of the type t. A text becomes a [time.Duration] through
// [time.ParseDuration]. Any other value goes through the JSON encoding of
// options: convert encodes the value and decodes the bytes into t. A
// decoder of a config file returns an int and an []any. The encoding turns
// them into an int64 and a []string. The JSON decoder calls the
// UnmarshalText of a type that implements [encoding.TextUnmarshaler], so
// such a type with a closed set of values refuses a text outside the set.
// The decoder also rejects a key that a struct of t does not declare.
//
// Error modes: convert wraps the error of ParseDuration, of the encoder or
// of the decoder. The message of the fault contains the option, the plugin
// and t.
func convert(name plugin.ID, key string, v any, t reflect.Type) (reflect.Value, error) {
	out := reflect.New(t)
	var err error
	if text, isText := v.(string); isText && t == durationType {
		var d time.Duration
		d, err = time.ParseDuration(text)
		out.Elem().Set(reflect.ValueOf(d))
	} else {
		var encoded []byte
		if encoded, err = json.Marshal(v); err == nil {
			dec := json.NewDecoder(bytes.NewReader(encoded))
			dec.DisallowUnknownFields()
			err = dec.Decode(out.Interface())
		}
	}
	if err != nil {
		return reflect.Value{}, fmt.Errorf("workspace: option %q of plugin %q takes %s: %w", key, name, t, err)
	}
	return out.Elem(), nil
}

// refine applies the plan refinements of the config to the declared plans
// before Build compiles them. It returns the refined plans and, apart, each
// disabled plan as the composition declares it. It replaces the sources and
// the layout fields that a refinement sets. It merges each refinement of a
// generator or a family over the refinement that the plan declares, through
// [layout.Refinement.Over]. It appends the generators that a refinement adds
// to a plan whose Extensible is true.
//
// refine returns an error for each of these refinements:
//
//   - a refinement of a plan that the composition does not declare
//   - a disabled plan that an enabled plan depends on or that a check
//     reads, with the names of both
//   - a disabled plan that lists a nil generator or declares no backend,
//     which the plan step does not check
//   - generators for a plan that is not extensible
//
// A disabled plan that refine refuses remains in the refined plans, so the
// later steps report no second error about it. refine copies each plan that
// it refines, so the declared plans of the composition do not change.
func refine(
	declared []Plan, refinements map[string]PlanConfig, checks []plugin.WorkspaceCheck,
) ([]Plan, []Plan, []error) {
	if len(refinements) == 0 {
		return declared, nil, nil
	}
	var faults []error
	for _, name := range slices.Sorted(maps.Keys(refinements)) {
		if !slices.ContainsFunc(declared, func(pl Plan) bool { return pl.Name == name }) {
			faults = append(faults, fmt.Errorf(
				"workspace: the config refines plan %q, which the composition does not declare", name,
			))
		}
	}
	needed := map[string]bool{}
	for _, pl := range declared {
		if refinements[pl.Name].Disabled {
			continue
		}
		for _, dep := range pl.DependsOn {
			if refinements[dep].Disabled {
				faults = append(faults, fmt.Errorf(
					"workspace: the config disables plan %q, and plan %q depends on it", dep, pl.Name,
				))
				needed[dep] = true
			}
		}
	}
	for _, c := range checks {
		if c == nil {
			continue
		}
		for _, name := range c.Reads() {
			if refinements[name].Disabled {
				faults = append(faults, fmt.Errorf(
					"workspace: the config disables plan %q, and check %s reads it", name, c.Name(),
				))
				needed[name] = true
			}
		}
	}
	out := make([]Plan, 0, len(declared))
	var disabled []Plan
	for _, pl := range declared {
		r := refinements[pl.Name]
		off := r.Disabled && !needed[pl.Name]
		if off {
			disabled = append(disabled, pl)
			if slices.Contains(pl.Generators, nil) {
				faults = append(faults, fmt.Errorf(
					"workspace: the config disables plan %q, which lists a nil generator", pl.Name,
				))
			}
			if pl.Backend == nil {
				faults = append(faults, fmt.Errorf(
					"workspace: the config disables plan %q, which declares no backend", pl.Name,
				))
			}
		}
		if len(r.Generators) > 0 {
			if pl.Extensible {
				pl.Generators = slices.Concat(pl.Generators, r.Generators)
			} else {
				faults = append(faults, fmt.Errorf(
					"workspace: the config adds generators to plan %q, which is not extensible", pl.Name,
				))
			}
		}
		if off {
			continue
		}
		if r.Sources != nil {
			pl.Sources = *r.Sources
		}
		if r.Policy != layout.PolicyInherit {
			pl.Layout.Policy = r.Policy
		}
		if r.Dir != "" {
			pl.Layout.Dir = r.Dir
		}
		if r.ImportBase != "" {
			pl.Layout.ImportBase = r.ImportBase
		}
		if len(r.Plugins) > 0 {
			pl.Layout.Plugins = maps.Clone(pl.Layout.Plugins)
			if pl.Layout.Plugins == nil {
				pl.Layout.Plugins = make(map[plugin.ID]layout.Refinement, len(r.Plugins))
			}
			for p, refinement := range r.Plugins {
				pl.Layout.Plugins[p] = refinement.Over(pl.Layout.Plugins[p])
			}
		}
		if len(r.Families) > 0 {
			pl.Layout.Families = maps.Clone(pl.Layout.Families)
			if pl.Layout.Families == nil {
				pl.Layout.Families = make(map[layout.Family]layout.Refinement, len(r.Families))
			}
			for f, refinement := range r.Families {
				pl.Layout.Families[f] = refinement.Over(pl.Layout.Families[f])
			}
		}
		out = append(out, pl)
	}
	return out, disabled, faults
}

// compilePlans is the plan step: every plan named once, at least one
// generator in a plan that is not extensible, exactly one backend
// against a registered target, every
// generator that declares templates serving that target, a layout the
// plan's generators' families admit, sources in the languages langs
// contains, dependencies on declared plans, and the roles fixed in
// turn order, which is the schedule the run executes as data. Each
// plan takes its backend's spoke and the policy of policies at its
// index. A plan that another plan depends on is marked exported.
func compilePlans(
	declared []Plan, gens []genEntry, targets map[plugin.Target]bool, langs map[symbol.Lang]bool,
	policies []plugin.Policy,
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
	for i, pl := range declared {
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
		if len(roles) == 0 && !pl.Extensible {
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q lists no generator", pl.Name,
			))
		}
		slices.SortFunc(roles, func(a, b genEntry) int {
			return cmp.Compare(a.turn, b.turn)
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
			backend: pl.Backend, policy: policies[i], routing: pl.Layout, outputs: outputs,
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

// stampable builds each plan's output contract from the brand and the
// backend's own comment syntax, and reports a syntax that cannot frame a
// generated file. It runs only where a composition declares output,
// because a run that writes nothing needs no layout, no render and no
// frame.
func stampable(plans []compiledPlan, brand output.Brand) []error {
	var faults []error
	for i := range plans {
		pl := &plans[i]
		if pl.backend == nil {
			continue // the plan's own fault is already collected
		}
		contract, err := output.NewContract(brand, pl.backend.Syntax())
		if err != nil {
			faults = append(faults, fmt.Errorf("workspace: plan %q: %w", pl.name, err))
			continue
		}
		pl.contract = contract
	}
	return faults
}
