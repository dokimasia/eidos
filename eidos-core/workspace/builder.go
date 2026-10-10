// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
)

// Config contains the values that Build applies to the composition. A
// command line decodes its config file into this struct, and this package
// defines no file format.
type Config struct {
	// Options contains the option values of the plugins. The outer key is
	// the name of a plugin, and the inner key is the key of an option.
	Options map[string]map[string]any
	// Plans contains the refinements of the plans. The key is the name of a
	// plan, and Build returns an error for a name that the composition does
	// not declare.
	Plans map[string]PlanConfig
	// Policies selects a choice for a policy key. The choice applies to
	// every plan whose backend declares the key. Build returns an error for
	// a key that no plan's backend declares, and for a choice outside the
	// key's choices.
	Policies map[plugin.PolicyKey]plugin.Choice
}

// PlanConfig is the refinement of one plan. Build applies it before it
// compiles the plan, so the fingerprint of the composition includes the
// refined sources and layout. Build leaves the plan unchanged for the zero
// PlanConfig.
type PlanConfig struct {
	// When Disabled is true, Build leaves the plan out of the composition.
	// Build returns an error for a disabled plan that an enabled plan
	// depends on or that a workspace check reads. The error includes the
	// names of both.
	Disabled bool
	// Build replaces the sources of the plan with Sources when Sources is
	// not nil.
	Sources *Sources
	// Build replaces the layout fields of the plan with Policy, Dir and
	// ImportBase when they are set. Policy is set when it is not
	// [layout.PolicyInherit], and Dir and ImportBase are set when they are
	// not empty. Build validates the refined layout in the same way as a
	// declared layout.
	Policy     layout.Policy
	Dir        string
	ImportBase string
	// Policies selects a choice for a policy key of the plan's backend,
	// and replaces the selection of Config.Policies for the plan. Build
	// returns an error for a key that the backend does not declare, and
	// for a choice outside the key's choices.
	Policies map[plugin.PolicyKey]plugin.Choice
}

// Plan is one write side of the composition. A plan is a value, not a
// plugin. Build validates the whole plan, so every name in an accepted plan
// refers to a registered plugin or target.
type Plan struct {
	// Name identifies the plan in the report of a run. Each plan of the
	// composition has a different name.
	Name string
	// Sources limits the packages that the generators of the plan run over.
	// The zero value includes every package.
	Sources Sources
	// DependsOn contains the names of the plans whose exports the
	// generators of the plan read. The plan generates after each of those
	// plans has rendered, and commits only when each of them commits. Build
	// returns an error for an undeclared name, for the name of the plan
	// itself, for a name listed twice and for a cycle. The error for a cycle
	// lists every plan of the cycle.
	DependsOn []string
	// The generators of the plan run in bucket order. Their order in the
	// list does not matter.
	Generators []plugin.Generator
	// Backend renders the settled store of the plan. Each plan has exactly
	// one backend, and its target must be registered.
	Backend plugin.Backend
	// Layout configures how the layout pass routes the declarations of the
	// plan to files. It contains the policy, the output directory, the
	// import base, and the refinements for each generator and each family.
	// The zero value writes every file beside its source. Build validates
	// the layout against the families that the generators of the plan
	// declare.
	Layout layout.Config
}

// Builder collects a composition. Each method appends or sets values and
// returns the builder. [Builder.Build] validates the composition, and
// returns all faults in one error.
//
// # Concurrency
//
// A Builder is not safe for concurrent use. A caller builds a composition
// on one goroutine.
//
// # Allocation contract
//
// [New] allocates the builder. A method that registers values appends them
// to a list. The first call allocates the list, and a later call allocates
// only to grow it. A method that sets one value allocates nothing.
// [Builder.Build] has its own allocation contract.
type Builder struct {
	frontends  []plugin.Frontend
	annotators []plugin.Annotator
	plans      []Plan
	checks     []plugin.WorkspaceCheck
	targets    []plugin.Target
	brand      output.Brand
	open       func() (output.Sink, error)
	ledger     func() (ledger.Ledger, error)
	id         string
	workers    int
	keys       []func(r *meta.Registry) error
	rules      []rules.SourceRules
	ignored    []directive.Name
	config     Config
	memo       Memo
}

// New returns an empty builder. It allocates once, for the builder.
func New() *Builder {
	return &Builder{}
}

// Brand sets the brand of the composition, and Build requires a brand. A
// carrier in the source opens with the brand, and the trailer of each
// stamped file records the brand as the owner of the file. A load excludes
// the files that the workspace stamped under its brand. A brand is a
// lowercase letter followed by lowercase letters, digits and hyphens. Brand
// allocates nothing.
func (b *Builder) Brand(brand output.Brand) *Builder {
	b.brand = brand
	return b
}

// BrandName returns the brand that the caller passed to Brand, or the empty
// Brand before the first call of Brand. A command line calls BrandName
// before Build to find its config file, whose name contains the brand.
// BrandName allocates nothing.
func (b *Builder) BrandName() output.Brand { return b.brand }

// Frontends registers the frontends that load the tree of a run, in load
// order.
//
// Build returns an error for each of these frontends:
//
//   - a nil frontend
//   - a frontend without a version, because each unit key includes the
//     version
//   - a frontend with an empty name, or with the name of another frontend
//     of the composition
//   - a frontend with the name of a kernel phase
//   - a frontend whose options the canonical encoding cannot encode in
//     full, because the fingerprint and each unit key include every option
//
// The frontend and the backend of one language can have the name of the
// language, as both Go plugins report under golang. A frontend that
// implements [plugin.KeyProvider] registers its language's keys under the
// language's spelling, in load order. The first call allocates the list of
// frontends, and a later call allocates only to grow it.
func (b *Builder) Frontends(fs ...plugin.Frontend) *Builder {
	b.frontends = append(b.frontends, fs...)
	return b
}

// Annotators registers the annotators, which stamp facts on the read side.
// The first call allocates the list of annotators, and a later call
// allocates only to grow it.
func (b *Builder) Annotators(as ...plugin.Annotator) *Builder {
	b.annotators = append(b.annotators, as...)
	return b
}

// Plans registers the plans, which are the write sides of the composition.
// The first call allocates the list of plans, and a later call allocates
// only to grow it.
func (b *Builder) Plans(ps ...Plan) *Builder {
	b.plans = append(b.plans, ps...)
	return b
}

// Checks registers the workspace checks that Close runs, in registration
// order. Build returns an error for a nil check, for a check with the name
// of another plugin, and for a check that reads an undeclared plan or lists
// one plan twice. A check takes options, keys and capabilities like any
// other plugin of the composition. The first call allocates the list of
// checks, and a later call allocates only to grow it.
func (b *Builder) Checks(cs ...plugin.WorkspaceCheck) *Builder {
	b.checks = append(b.checks, cs...)
	return b
}

// Output sets the function that opens the sink of a plan. open returns a
// new sink or an error. A run calls open once for each plan that it stages,
// after the render, so each plan stages into its own sink. The plans call
// open from their own goroutines, so open must be safe for concurrent use.
// A run fails when open returns neither a sink nor an error.
//
// A plan opens no sink when it reports an Error, or when the shared phases
// of the run reported an Error. The previous generation of the files of
// such a plan remains in place.
//
// A composition without an output stops after the settle. The emit stores
// of its plans are then the whole product of a run, so the composition
// computes what it would write and writes nothing. Output allocates
// nothing.
func (b *Builder) Output(open func() (output.Sink, error)) *Builder {
	b.open = open
	return b
}

// Ledger sets the function that opens the ledger. A run reads the record of
// the previous run from the ledger, and writes its own record to it. open
// returns a new ledger. A run of a composition with an output calls open
// once, before the load. A composition without a ledger runs as if no run
// had committed, so it removes no file and records nothing. Ledger
// allocates nothing.
func (b *Builder) Ledger(open func() (ledger.Ledger, error)) *Builder {
	b.ledger = open
	return b
}

// Workspace sets the name of the workspace, which the ledger records in its
// manifest. When id is empty, the ledger chooses the name, and the disk
// ledger records the base name of the workspace root. Workspace allocates
// nothing.
func (b *Builder) Workspace(id string) *Builder {
	b.id = id
	return b
}

// Parallel sets the number of invocations that one phase call runs at the
// same time. The invocations are the matches of an annotator or a
// generator inside its bucket. With 0 or 1, the default, a phase call runs
// one match at a time, and Build returns an error for a negative count.
//
// The output does not depend on the count. A phase call applies its
// placements, slot appends and findings in canonical match order, and the
// fact store ranks stamps by that order. Plans run in parallel at any
// count. Parallel allocates nothing.
func (b *Builder) Parallel(workers int) *Builder {
	b.workers = workers
	return b
}

// Targets registers the targets of the composition. Build resolves the
// target of each backend against them. The first call allocates the list
// of targets, and a later call allocates only to grow it.
func (b *Builder) Targets(ts ...plugin.Target) *Builder {
	b.targets = append(b.targets, ts...)
	return b
}

// Keys registers metadata keys that belong to the composition and to no
// plugin, such as the keys of a consumer or of a test fixture. Build runs
// the registrations in declaration order, through the handle of the
// composition on the registry, after every frontend, target and plugin
// registered. The keys of a composed language belong to the language, so
// Build refuses the composition's own claim of the language's namespace.
// The first call allocates the list of registrations, and a later call
// allocates only to grow it.
func (b *Builder) Keys(register ...func(r *meta.Registry) error) *Builder {
	b.keys = append(b.keys, register...)
	return b
}

// Rules registers the language rules for the walks of the kernel, one
// value for each language. A run binds the rules of a subject by its
// language. For a language without rules, a run binds the absent rules and
// reports a warning. Build returns an error for two values of one language.
// The first call allocates the list of rules, and a later call allocates
// only to grow it.
func (b *Builder) Rules(rs ...rules.SourceRules) *Builder {
	b.rules = append(b.rules, rs...)
	return b
}

// Config sets the config that Build applies to the composition. It
// replaces the config of an earlier call, and it allocates nothing.
func (b *Builder) Config(c Config) *Builder {
	b.config = c
	return b
}

// Ignore adds directive names that the load does not report as unclaimed.
// An unclaimed directive is written under the brand for a plugin that the
// composition does not include. A full name matches one directive, and a
// plugin prefix that ends with a colon, such as "legacy:", matches every
// directive of that plugin. Build returns an error for a name that a
// registered schema claims, because the load would then skip the
// validation of a registered directive. The first call allocates the list
// of names, and a later call allocates only to grow it.
func (b *Builder) Ignore(names ...directive.Name) *Builder {
	b.ignored = append(b.ignored, names...)
	return b
}

// Memo configures the parse memo. The default is the zero Memo, which turns
// the memo off. Build returns an error for a negative limit. Memo allocates
// nothing.
func (b *Builder) Memo(m Memo) *Builder {
	b.memo = m
	return b
}

// Build runs every step and returns the immutable workspace, or one error
// that joins every fault. Each step runs even when an earlier step found
// faults, so the error lists every fault. The exception is a fault that
// makes a later check of the same item meaningless, and Build skips that
// check. A composition without a valid brand is a fault, because the load
// reads carriers under the brand and the output stamps files under it.
//
// The configure step writes the values of the config into the options
// struct of each plugin, so a plugin instance belongs to one workspace.
// When one instance is built into two workspaces, both read the values of
// the second. Build computes the fingerprint over the values
// that the config set.
//
// # Allocation contract
//
// Build allocates in proportion to the composition. It allocates the
// registries, the roster, the capability order, the plans' policies, the
// compiled plans and checks, the workspace and the fingerprint. A
// composition of 64 annotators and 8 plans of 4 generators allocates 935
// times.
func (b *Builder) Build() (*Workspace, error) {
	faults := b.brandFaults()
	if b.workers < 0 {
		faults = append(faults, fmt.Errorf(
			"workspace: Parallel(%d) is negative, and a phase call runs at least one invocation at a time",
			b.workers,
		))
	}
	if b.memo.Limit < 0 {
		faults = append(faults, fmt.Errorf(
			"workspace: the memo limit %d is negative, and a limit of 0 turns the memo off", b.memo.Limit,
		))
	}
	faults = append(faults, frontendFaults(b.frontends)...)
	fronts, ferr := frontendOptions(b.frontends)
	faults = append(faults, ferr...)
	roster, byName, afaults := b.assemble()
	faults = append(faults, afaults...)
	refined, refineFaults := refine(b.plans, b.config.Plans, b.checks)
	faults = append(faults, refineFaults...)
	policies, broken, pfaults := resolvePolicies(refined, b.config)
	faults = append(faults, pfaults...)
	entries, efaults := lowerings(refined, byName, broken)
	faults = append(faults, efaults...)
	reg, rerr := b.register(roster, entries)
	faults = append(faults, rerr...)
	ann, gens, lerr := lower(roster, entries)
	faults = append(faults, lerr...)
	faults = append(faults, bindKeys(roster, ann, reg.keys)...)
	options, cerr := configure(roster, byName, b.config)
	faults = append(faults, cerr...)
	plans, perr := compilePlans(refined, gens, reg.targets, languages(b.frontends, reg.rules), policies)
	faults = append(faults, perr...)
	order, oerr := orderPlans(plans)
	faults = append(faults, oerr...)
	checks, kerr := compileChecks(b.checks, plans)
	faults = append(faults, kerr...)
	if b.open != nil && b.brand.Valid() {
		faults = append(faults, stampable(plans, b.brand)...)
	}
	if len(faults) > 0 {
		return nil, errors.Join(faults...)
	}
	return &Workspace{
		keys:        reg.keys,
		kernel:      reg.kernel,
		directives:  reg.directives,
		rules:       reg.rules,
		annotate:    ann,
		plans:       plans,
		order:       order,
		checks:      checks,
		frontends:   slices.Clone(b.frontends),
		contracts:   contractsOf(reg.keys),
		open:        b.open,
		ledger:      b.ledger,
		id:          b.id,
		workers:     b.workers,
		brand:       b.brand,
		memo:        b.memo,
		fingerprint: b.fingerprint(ann, plans, checks, options, fronts, reg.keys),
		trees:       templateTrees(plans),
	}, nil
}

// brandFaults runs the brand step. It returns an error when the composition
// has no brand, or when [output.Brand.Valid] rejects the brand.
func (b *Builder) brandFaults() []error {
	switch {
	case b.brand == "":
		return []error{errors.New(
			"workspace: the composition declares no brand, and the load and the output require one",
		)}
	case !b.brand.Valid():
		return []error{fmt.Errorf(
			"workspace: %q is not a brand: use a lowercase letter followed by lowercase letters, digits and hyphens",
			string(b.brand),
		)}
	}
	return nil
}
