// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// loweringSuffix follows the target's spelling in the name of a
// lowering entry. Rule 0 of an entry stamps the names of one package,
// and rule 1 stamps the values of one instance of the target's
// directive.
const (
	loweringSuffix               = "-lowering"
	rulePackage    plugin.RuleID = 0
	ruleInstance   plugin.RuleID = 1
)

// The docs of a target's name key, of the name param of the target's
// directive and of the directive. Each is a format of the target's
// spelling.
const (
	nameKeyDoc   = "records a declaration's name in the %s target"
	nameParamDoc = "the declaration's name in the %s target"
	directiveDoc = "overrides how the %s target spells the declaration"
)

// lowering is the annotate entry of one rendering target whose backend
// respells names or declares lowering policies. Its name is the
// target's spelling followed by -lowering. It runs before every
// annotator of the composition, and it stamps two kinds of fact:
//
//   - Rule 0 stamps the target's name key on each file-level
//     declaration, field, method, enum variant and sum variant of an
//     admitted package. A plan of the target admits such a package, its
//     language is not the target's, and no store provides it. The value
//     is the backend's respell of the declared name at the declaration's
//     own kind and visibility. The claim has plugin authority, the
//     package as the subject of its order, and the declaration as its
//     derivation. A target without nested types declares a nested type
//     in the file, so the value of a nested type is the respell of its
//     flat name, [symbol.Identity.FlatName], at the kind and the
//     visibility of a declaration of the file.
//   - Rule 1 stamps the values of each instance of the target's
//     directive on the instance's subject. The name param stamps the
//     name key, and each policy param stamps the key of its policy. The
//     claim has directive authority, the instance's position, and the
//     instance in its order.
//
// Rule 0 leaves out a declaration that a skip excludes from the entry
// and one whose name the respell hook refuses. Where two declarations
// of a package share an identity, the first in the package's order sets
// the stamp. A backend that does not respell names leaves rule 0
// without a match.
//
// The entry journals one invocation for each package that rule 0
// admits, which reads the package whole, and one for each instance of
// the directive. It honours the selection of a warm run as the
// authoring kit does. It runs each listed match that its gates still
// admit, and every match of each candidate. A warm run lists the
// package match of each package whose members or files changed.
//
// # Concurrency
//
// A lowering is immutable once Build returns. Each call keeps its state
// in a value of its own, so the runs of one workspace call one lowering
// concurrently.
//
// # Allocation contract
//
// A call allocates its state, its read set and its reader. A package
// that rule 0 stamps allocates one array of its claims' derivations, and
// a respelled name that differs from the declared name allocates its
// spelling. Each stamp allocates what the fact store keeps of a claim.
type lowering struct {
	name      plugin.ID
	target    plugin.Target
	directive directive.Name
	// respell is the backend's respell hook, and nil where the backend
	// does not respell names.
	respell plugin.Respeller
	// policies are the backend's lowering policies, each with its param
	// and its key, and nil where the backend declares a defective one.
	policies []policyParam
	// scopes are the sources of the enabled plans of the target.
	scopes []planScope
	// nameKey is the handle of the target's name key, which the register
	// step sets where the backend respells.
	nameKey meta.Key[string]
}

var _ plugin.Annotator = (*lowering)(nil)

// policyParam is one lowering policy of a target. It has the policy's
// spec, the param of the target's directive that overrides the policy,
// and the handle of the policy's key, which the register step sets.
type policyParam struct {
	spec  plugin.PolicySpec
	param directive.ParamKey
	key   meta.Key[string]
}

// planScope is the sources of one plan, with their patterns parsed.
type planScope struct {
	sources Sources
	dirs    []directory
}

// lowerings is the lowering step. It returns one lowering entry for
// each target of an enabled plan whose backend respells names or
// declares lowering policies. The entries are in the order of their
// names. Each entry keeps the sources of every enabled plan of its
// target, and the backend's policies where broken does not list the
// backend. The step returns an error that lists the backends for two
// backends of one such target. It returns the roster's error of a
// duplicate name for an entry whose name a plugin of the roster has. A
// composition without such a target allocates nothing in the step.
func lowerings(plans []Plan, byName map[plugin.ID]plugin.Plugin, broken map[plugin.ID]bool) ([]*lowering, []error) {
	var out []*lowering
	var faults []error
	var seen []plugin.Target
	for _, pl := range plans {
		b := pl.Backend
		if b == nil || slices.Contains(seen, b.Target()) {
			continue
		}
		respell, respells := b.(plugin.Respeller)
		if !respells && len(policiesOf(b)) == 0 {
			continue
		}
		t := b.Target()
		seen = append(seen, t)
		l := &lowering{
			name: plugin.ID(string(t) + loweringSuffix), target: t, directive: directive.Name(t), respell: respell,
		}
		var backends []string
		for _, other := range plans {
			if other.Backend == nil || other.Backend.Target() != t {
				continue
			}
			if name := string(other.Backend.Name()); !slices.Contains(backends, name) {
				backends = append(backends, name)
			}
			l.scopes = append(l.scopes, planScope{sources: other.Sources, dirs: other.Sources.directories()})
		}
		if len(backends) > 1 {
			faults = append(faults, fmt.Errorf(
				"workspace: target %q renders through %s, and a target whose backend respells names or "+
					"declares policies renders through one backend", t, strings.Join(backends, " and "),
			))
			continue
		}
		if _, taken := byName[l.name]; taken {
			faults = append(faults, fmt.Errorf("workspace: two plugins return the name %q", l.name))
			continue
		}
		if !broken[b.Name()] {
			for _, spec := range policiesOf(b) {
				l.policies = append(l.policies, policyParam{spec: spec, param: spec.Key.Param()})
			}
		}
		out = append(out, l)
	}
	slices.SortFunc(out, func(a, b *lowering) int { return cmp.Compare(a.name, b.name) })
	return out, faults
}

// policiesOf returns the lowering policies that a backend declares, and
// nil for a backend that does not implement [plugin.PolicyProvider].
func policiesOf(b plugin.Backend) []plugin.PolicySpec {
	if pp, declares := b.(plugin.PolicyProvider); declares {
		return pp.Policies()
	}
	return nil
}

// Name returns the entry's name, the target's spelling followed by
// -lowering. It allocates nothing.
func (l *lowering) Name() plugin.ID { return l.name }

// Annotate runs the entry's rules: every match where the call has no
// selection, and the matches that the selection admits otherwise, by
// the rules of [lowering]. It journals each invocation in canonical match
// order, and then each candidate's matches. The workspace hands every
// call a journal.
//
// Error modes: the fact store's errors for the stamps that it refuses,
// joined, each a defect of the composition. The call runs every match
// beside them.
func (l *lowering) Annotate(ctx *plugin.AnnotatorContext) error {
	c := &lowerCall{l: l, ctx: ctx, reads: store.NewReadSet()}
	// The index refused an unfrozen graph at its construction, and the
	// read set is not nil, so the reader mints.
	c.reader, _ = ctx.Index.Reader(c.reads)
	if ctx.Select == nil {
		return c.all()
	}
	return c.selected(ctx.Select)
}

// register registers the target's keys under the target's spelling.
// These are the name key where the backend respells, and the key of each
// policy. It keeps each handle, which the entry stamps through, and
// returns the registry's errors.
func (l *lowering) register(keys *meta.Registry) []error {
	r := keys.For(string(l.target))
	if err := r.ClaimNamespace(string(l.target)); err != nil {
		return []error{err}
	}
	var faults []error
	if l.respell != nil {
		key, err := meta.Register[string](r, meta.KeySpec{
			Name: l.target.NameKey(), Doc: fmt.Sprintf(nameKeyDoc, l.target),
		})
		if err != nil {
			faults = append(faults, err)
		}
		l.nameKey = key
	}
	for i := range l.policies {
		p := &l.policies[i]
		key, err := meta.Register[string](r, meta.KeySpec{Name: meta.KeyName(p.spec.Key), Doc: p.spec.Doc})
		if err != nil {
			faults = append(faults, err)
		}
		p.key = key
	}
	return faults
}

// schema returns the target's directive, a kernel schema whose name is
// the target's spelling. The directive has a name param where the
// backend respells, and one param for each policy under the policy's
// name. Each policy's param takes the policy's choices alone.
func (l *lowering) schema() directive.Schema {
	s := directive.Schema{Name: l.directive, Doc: fmt.Sprintf(directiveDoc, l.target)}
	if l.respell != nil {
		s.Params = append(s.Params, directive.ParamSpec{
			Key: plugin.NameParam, Type: directive.TypeString, Doc: fmt.Sprintf(nameParamDoc, l.target),
		})
	}
	for _, p := range l.policies {
		choices := make([]string, len(p.spec.Choices))
		for i, c := range p.spec.Choices {
			choices[i] = string(c)
		}
		s.Params = append(s.Params, directive.ParamSpec{
			Key: p.param, Type: directive.TypeString, Choices: choices, Doc: p.spec.Doc,
		})
	}
	return s
}

// admits reports whether a plan of the target admits a package. No store
// provides such a package, and the sources of an enabled plan of the
// target admit its language and contain it.
func (l *lowering) admits(p *node.Package, facts *meta.Facts, k meta.KernelKeys) bool {
	for _, f := range p.Files {
		if _, _, stored := plugin.CutStorePath(f.Path); stored {
			return false
		}
	}
	for _, s := range l.scopes {
		if (s.sources.Lang == "" || s.sources.Lang == p.ID.Lang) && s.sources.contains(p, s.dirs, facts, k) {
			return true
		}
	}
	return false
}

// lowerCall is the state of one call of a lowering entry: the call's
// reader, which records into reads, and the buffers that each
// invocation reuses.
//
// # Concurrency
//
// A lowerCall is not safe for concurrent use. One call runs its
// invocations one at a time.
type lowerCall struct {
	l      *lowering
	ctx    *plugin.AnnotatorContext
	reads  *store.ReadSet
	reader *store.Reader
	// stamps are the names of the package that rule 0 runs on, and
	// claimed are the facts of the running invocation, sorted.
	stamps  []nameStamp
	claimed []meta.FactRef
	// keys are the matches of one candidate, which the call reports.
	keys []plugin.MatchKey
}

// nameStamp is one name that rule 0 stamps: the declaration, and the
// target's spelling of its name.
type nameStamp struct {
	subject symbol.Identity
	name    string
}

// all runs every match of the entry in canonical match order: rule 0 on
// each package of the graph, and then rule 1 on each instance of the
// target's directive.
func (c *lowerCall) all() error {
	var err error
	if c.l.respell != nil {
		for s := range c.ctx.Index.ByKind(symbol.KindPackage) {
			// The graph indexes declarations alone.
			p, _ := s.(node.Declaration)
			_, perr := c.pkg(p.Identity())
			err = errors.Join(err, perr)
		}
	}
	for s := range c.ctx.Index.ByDirective(c.l.directive) {
		d, _ := s.(node.Declaration)
		id := d.Identity()
		ds := c.ctx.Index.DirectivesOf(id)
		for i := range ds {
			if ds[i].Name == c.l.directive {
				err = errors.Join(err, c.instance(id, &ds[i]))
			}
		}
	}
	return err
}

// selected runs the matches that a warm run's selection admits, in
// canonical match order. Rule 0 runs on each package that the selection
// lists or that is a candidate. Rule 1 runs on each listed instance that
// its subject still has, and on every instance of each candidate. The
// call then reports the matches of each candidate.
func (c *lowerCall) selected(sel *plugin.Selection) error {
	var pkgs []symbol.Identity
	var instances []plugin.MatchKey
	for _, m := range sel.Matches {
		switch m.Rule {
		case rulePackage:
			pkgs = append(pkgs, m.Subject)
		case ruleInstance:
			instances = append(instances, m)
		case plugin.WholeCall:
			// The lowering journals an invocation for each match, so its
			// selection has no whole call.
		}
	}
	for _, id := range sel.Candidates {
		if id.Kind == symbol.KindPackage {
			pkgs = append(pkgs, id)
		}
		for _, d := range c.ctx.Index.DirectivesOf(id) {
			if d.Name == c.l.directive {
				instances = append(instances, plugin.MatchKey{
					Plugin: c.ctx.Plugin, Rule: ruleInstance, Subject: id, Instance: d.Instance,
				})
			}
		}
	}
	slices.SortFunc(pkgs, symbol.Identity.Compare)
	slices.SortFunc(instances, plugin.MatchKey.Compare)
	var err error
	var matched []plugin.MatchKey
	if c.l.respell != nil {
		for _, id := range slices.Compact(pkgs) {
			ran, perr := c.pkg(id)
			err = errors.Join(err, perr)
			if ran {
				matched = append(matched, plugin.MatchKey{Plugin: c.ctx.Plugin, Rule: rulePackage, Subject: id})
			}
		}
	}
	for _, m := range slices.Compact(instances) {
		ds := c.ctx.Index.DirectivesOf(m.Subject)
		at := slices.IndexFunc(ds, func(d directive.Directive) bool {
			return d.Name == c.l.directive && d.Instance == m.Instance
		})
		if at >= 0 {
			err = errors.Join(err, c.instance(m.Subject, &ds[at]))
			matched = append(matched, m)
		}
	}
	c.evaluated(sel.Candidates, matched)
	return err
}

// evaluated reports the matches of each candidate to the call's journal,
// in the order of the candidates, and nil for a candidate without one.
func (c *lowerCall) evaluated(candidates []symbol.Identity, matched []plugin.MatchKey) {
	for _, id := range candidates {
		c.keys = c.keys[:0]
		for _, m := range matched {
			if m.Subject == id {
				c.keys = append(c.keys, m)
			}
		}
		if len(c.keys) == 0 {
			c.ctx.Journal.Evaluated(id, nil)
			continue
		}
		c.ctx.Journal.Evaluated(id, c.keys)
	}
}

// pkg runs rule 0 on a package where a plan of the target admits it. It
// stamps the target's name on each declaration that the rule names, and
// journals the invocation, which reads the package whole. It reports
// whether the rule matched the package.
//
// Error modes: the fact store's errors for the stamps that it refuses,
// joined.
func (c *lowerCall) pkg(id symbol.Identity) (bool, error) {
	if id.Lang == symbol.Lang(c.l.target) {
		return false, nil
	}
	c.reads.Reset()
	// A package that the graph does not contain looks up as nil.
	s, _ := c.reader.Lookup(id)
	p, is := s.(*node.Package)
	if !is || !c.l.admits(p, c.ctx.Facts, c.ctx.Kernel) {
		return false, nil
	}
	c.stamps = c.stamps[:0]
	for _, f := range p.Files {
		for _, d := range f.Decls {
			c.collect(symbol.KindInvalid, d, false)
		}
	}
	slices.SortStableFunc(c.stamps, func(a, b nameStamp) int { return a.subject.Compare(b.subject) })
	c.stamps = slices.CompactFunc(c.stamps, func(a, b nameStamp) bool { return a.subject == b.subject })
	derived := make([]meta.Read, len(c.stamps))
	order := meta.Order{Rule: int(rulePackage), Subject: id}
	c.claimed = c.claimed[:0]
	var err error
	for i, n := range c.stamps {
		derived[i] = meta.Read{Subject: n.subject}
		claim := meta.Claim{
			Subject: n.subject, Authority: meta.AuthorityPlugin, Bucket: c.ctx.Bucket, Plugin: c.ctx.Plugin,
			Order: order, Derived: derived[i : i+1 : i+1],
		}
		err = errors.Join(err, c.write(c.l.nameKey, n.name, claim))
	}
	c.journal(plugin.MatchKey{Plugin: c.ctx.Plugin, Rule: rulePackage, Subject: id}, c.reads)
	return true, err
}

// collect adds the stamps that rule 0 makes in one declaration of a
// file: the stamps of its fields, methods and variants, each under its
// host's kind, and its own stamp. A nested type takes the stamp of its
// flat name at the kind of a declaration of the file. A symbol of another
// kind adds none.
func (c *lowerCall) collect(host symbol.Kind, s symbol.Symbol, nested bool) {
	var (
		id   symbol.Identity
		kind symbol.Kind
		v    symbol.Visibility
		name string
	)
	switch x := s.(type) {
	case *node.Struct:
		id, kind, v, name = x.ID, symbol.KindStruct, x.Visibility, x.Name
		c.members(kind, x.Fields, x.Methods)
		for _, t := range x.Types {
			c.collect(kind, t, true)
		}
	case *node.Interface:
		id, kind, v, name = x.ID, symbol.KindInterface, x.Visibility, x.Name
		c.members(kind, x.Fields, x.Methods)
		for _, t := range x.Types {
			c.collect(kind, t, true)
		}
	case *node.Enum:
		id, kind, v, name = x.ID, symbol.KindEnum, x.Visibility, x.Name
		for _, variant := range x.Variants {
			c.add(kind, variant.ID, symbol.KindEnumVariant, 0, variant.Name)
		}
		c.members(kind, x.Fields, x.Methods)
	case *node.Sum:
		id, kind, v, name = x.ID, symbol.KindSum, x.Visibility, x.Name
		for _, variant := range x.Variants {
			c.add(kind, variant.ID, symbol.KindSumVariant, 0, variant.Name)
			c.members(symbol.KindSumVariant, variant.Fields, nil)
		}
		c.members(kind, nil, x.Methods)
	case *node.Function:
		id, kind, v, name = x.ID, symbol.KindFunction, x.Visibility, x.Name
	case *node.Method:
		id, kind, v, name = x.ID, symbol.KindMethod, x.Visibility, x.Name
	case *node.Alias:
		id, kind, v, name = x.ID, symbol.KindAlias, x.Visibility, x.Name
	case *node.Constant:
		id, kind, v, name = x.ID, symbol.KindConstant, x.Visibility, x.Name
	case *node.Variable:
		id, kind, v, name = x.ID, symbol.KindVariable, x.Visibility, x.Name
	}
	if nested {
		c.add(symbol.KindInvalid, id, kind, v, id.FlatName())
		return
	}
	c.add(host, id, kind, v, name)
}

// members adds the stamps of the fields and the methods of one host.
func (c *lowerCall) members(host symbol.Kind, fields []*node.Field, methods []*node.Method) {
	for _, f := range fields {
		c.add(host, f.ID, symbol.KindField, f.Visibility, f.Name)
	}
	for _, m := range methods {
		c.add(host, m.ID, symbol.KindMethod, m.Visibility, m.Name)
	}
}

// add adds the stamp of one declaration: the respell of its name at its
// host's kind, its own kind and its visibility. A declaration without an
// identity or a name, one that a skip excludes from the entry, and one
// whose name the respell hook refuses add none.
func (c *lowerCall) add(host symbol.Kind, id symbol.Identity, kind symbol.Kind, v symbol.Visibility, name string) {
	if id.IsZero() || name == "" || c.ctx.Index.Skipped(id, c.ctx.Plugin) {
		return
	}
	spelled, err := c.l.respell.Respell(host, kind, v, name)
	if err != nil {
		return
	}
	c.stamps = append(c.stamps, nameStamp{subject: id, name: spelled})
}

// instance runs rule 1 on one instance of the target's directive. It
// stamps the instance's name and policy values on the subject at
// directive authority, and journals the invocation.
//
// Error modes: the fact store's errors for the stamps that it refuses,
// joined.
func (c *lowerCall) instance(id symbol.Identity, d *directive.Directive) error {
	claim := meta.Claim{
		Subject: id, Authority: meta.AuthorityDirective, Bucket: c.ctx.Bucket, Plugin: c.ctx.Plugin,
		Order: meta.Order{Rule: int(ruleInstance), Subject: id, Instance: d.Instance}, Pos: d.Pos,
	}
	c.claimed = c.claimed[:0]
	var err error
	if v, written := d.Param(plugin.NameParam); written {
		err = c.write(c.l.nameKey, v.Str, claim)
	}
	for _, p := range c.l.policies {
		if v, written := d.Param(p.param); written {
			err = errors.Join(err, c.write(p.key, v.Str, claim))
		}
	}
	slices.SortFunc(c.claimed, meta.FactRef.Compare)
	c.journal(plugin.MatchKey{Plugin: c.ctx.Plugin, Rule: ruleInstance, Subject: id, Instance: d.Instance}, nil)
	return err
}

// write stamps one value of the running invocation, and adds the fact to
// the invocation's claims where the fact store keeps the stamp.
//
// Error modes: the fact store's error for a stamp that it refuses.
func (c *lowerCall) write(k meta.Key[string], value string, claim meta.Claim) error {
	err := meta.Stamp(c.ctx.Facts, k, value, claim)
	if err == nil {
		c.claimed = append(c.claimed, meta.FactRef{Subject: claim.Subject, Key: k.Name()})
	}
	return err
}

// journal hands the record of the running invocation to the call's
// journal: its match, its reads and its claims.
func (c *lowerCall) journal(m plugin.MatchKey, reads *store.ReadSet) {
	c.ctx.Journal.Invoked(plugin.Invocation{Match: m, Reads: reads, Claimed: c.claimed})
}
