// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Input is one plan's routing input.
type Input struct {
	// Emit is the plan's settled store.
	Emit *plugin.Emit
	// Config is the plan's configuration, which [Config.Check]
	// accepted.
	Config Config
	// Outputs are the families each generator of the plan declares,
	// keyed by generator.
	Outputs map[plugin.ID][]plugin.Output
	// Speller is the backend's filename half.
	Speller plugin.FileSpeller
	// Packager is the backend's package half. A nil Packager gives
	// every file the package of its first unit.
	Packager plugin.Packager
	// Index resolves origins: their source files, their packages and
	// their validated directives.
	Index *plugin.Index
	// Directives names the plugin that registered each directive's
	// schema, which scopes the reserved routing keys. A nil registry
	// leaves the kernel out directive as the only override.
	Directives *directive.Registry
	// Residents returns the source files that the load placed in one
	// directory, sorted by file. Where the function is nil, no directory
	// has residents.
	Residents func(dir string) []plugin.Resident
	// Modules are the toolchain modules the load resolved, innermost
	// root first.
	Modules []plugin.Module
	// Others are the names of the plan's files that the run keeps
	// without routing them again, nil for a store that contains the whole
	// plan. A bare reference to a name of a kept file qualifies with the
	// package of that file.
	Others plugin.Names
	// Target is the plan's target, the target of the backend that settled
	// Emit. A type reference whose target is a declaration of another
	// language is a translated reference, which takes the package of the
	// file that declares its referent. Route treats a reference as a
	// translated one only where Target is set.
	Target plugin.Target
	// Sink takes the routing findings.
	Sink *diag.Sink
}

// check returns the defect in an input Route cannot route, which is a
// fault of the caller and never a finding.
func (in Input) check() error {
	switch {
	case in.Emit == nil:
		return errors.New("layout: no emit store to route")
	case !in.Emit.Settled():
		return errors.New("layout: the store is unsettled, and routing reads the settled names")
	case in.Speller == nil:
		return errors.New("layout: no filename speller to name the files")
	case in.Index == nil:
		return errors.New("layout: no index to resolve the origins through")
	case in.Sink == nil:
		return errors.New("layout: no sink to report the routing findings into")
	}
	return nil
}

// Route resolves every declaration of one settled plan to the file it
// is written in, and returns the files sorted by path. Each file's
// units keep the store's unit order, and each unit keeps its
// declarations' order. A declaration that a finding refuses is absent
// from every file, and a file left with no declaration is absent too.
//
// Route reports routing problems to the sink under [diag.PhaseLayout],
// each an Error. It sets the package of every bare reference and every
// translated reference that crosses into a file of another package, in
// place. It returns an
// error only for a defect in its inputs: a nil store, an unsettled
// store, a missing speller, index or sink, a speller whose parts do not
// contain the declarations of the unit it split, and a filename that is
// not one path element.
//
// # Cost
//
// Route reads the overrides on each declaration's origin once, and
// visits a unit's declarations again only where one of them has an
// override. A unit without one joins its file whole, without a copy,
// so no declaration allocates. Each file costs its path, and the
// target's filename and split of the unit it comes from.
func Route(in Input) ([]plugin.File, error) {
	if err := in.check(); err != nil {
		return nil, err
	}
	// A plan routes about one bucket and one file per unit, so the
	// unit count sizes both.
	n := 0
	for range in.Emit.Units() {
		n++
	}
	r := &router{
		in:      in,
		units:   slices.AppendSeq(make([]plugin.Unit, 0, n), in.Emit.Units()),
		dirs:    map[symbol.Identity]packageDir{},
		buckets: make([]bucket, 0, n),
		at:      make(map[destKey]int, n),
		files:   make([]building, 0, n),
		byPath:  make(map[string]int, n),
		refused: map[symbol.Symbol]struct{}{},
	}
	r.readOverrides()
	r.place()
	if err := r.name(); err != nil {
		return nil, err
	}
	r.packages()
	r.collide()
	r.qualify()
	return r.assemble(), nil
}

// router is one plan's routing state.
type router struct {
	in    Input
	units []plugin.Unit
	// overrides contains the routing written on each declaration's
	// origin, keyed by the declaration's position in store order, for
	// the declarations with any.
	overrides map[int]override
	// spread collects the families of each origin whose override names
	// a path without a tag, the only origins an ambiguity can concern.
	spread map[placement]map[string]struct{}
	// dirs caches each package's source directory.
	dirs map[symbol.Identity]packageDir
	// buckets are the destinations in the order their first
	// declaration arrived, and at indexes them.
	buckets []bucket
	at      map[destKey]int
	// files are the routed files, sorted by path, and byPath indexes
	// them.
	files  []building
	byPath map[string]int
	// refused contains the declarations a finding refused.
	refused map[symbol.Symbol]struct{}
}

// placement addresses one plugin's declarations of one origin.
type placement struct {
	origin symbol.Identity
	plugin plugin.ID
}

// packageDir is one package's source directory, and whether the
// package has a workspace source.
type packageDir struct {
	dir   string
	found bool
}

// destination is where one declaration is written, before its file is
// named: the unit it joins and the directory, with the filename that an
// override or a refinement fixes, empty where the target spells it.
type destination struct {
	plugin plugin.ID
	tag    string
	per    plugin.Cardinality
	word   string
	key    string
	pkg    symbol.Identity
	dir    string
	file   string
}

// index returns the fields that tell two destinations apart: a
// family's cardinality and word follow from its plugin and tag, and a
// unit's package from its language and path. The key is under the
// size a map stores inline, so creating a bucket allocates nothing for
// its key.
func (d destination) index() destKey {
	return destKey{
		plugin: d.plugin, tag: d.tag, key: d.key,
		lang: d.pkg.Lang, pkg: d.pkg.Package, dir: d.dir, file: d.file,
	}
}

// destKey indexes a destination's bucket.
type destKey struct {
	plugin plugin.ID
	tag    string
	key    string
	lang   symbol.Lang
	pkg    string
	dir    string
	file   string
}

// bucket is the declarations of one destination, in store order.
type bucket struct {
	dest  destination
	decls []symbol.Symbol
	// first is the first source unit that contributed, and more lists
	// the later ones, in the order they first contributed.
	first int
	more  []int
	// whole reports that every contribution was a whole source unit.
	whole bool
	// shared reports that decls is a source unit's own slice, which an
	// append copies first.
	shared bool
}

// building is one routed file under construction. reason is the
// target's error where it derives no package for the file.
type building struct {
	file   plugin.File
	reason error
}

// readOverrides reads the routing on every declaration's origin once,
// before any declaration is placed, because an ambiguity depends on
// every family an origin's declarations are placed in.
func (r *router) readOverrides() {
	k := 0
	for i := range r.units {
		u := &r.units[i]
		_, declared := familyOf(r.in.Outputs[u.Plugin], u.Tag)
		for _, d := range u.Decls {
			k++
			if !declared {
				continue
			}
			origin, _ := emit.OriginOf(d)
			ov := r.overrideOf(origin, u.Plugin)
			if ov == (override{}) {
				continue
			}
			if r.overrides == nil {
				r.overrides = map[int]override{}
			}
			r.overrides[k-1] = ov
			if ov.path == "" || ov.tag != "" {
				continue
			}
			p := placement{origin: origin, plugin: u.Plugin}
			if r.spread == nil {
				r.spread = map[placement]map[string]struct{}{}
			}
			if r.spread[p] == nil {
				r.spread[p] = map[string]struct{}{}
			}
			r.spread[p][u.Tag] = struct{}{}
		}
	}
}

// place decides every declaration's destination and gathers the
// declarations into one bucket per destination. A unit whose family
// its plugin does not declare, and a declaration a finding refuses,
// join no bucket. A unit without an overridden declaration routes
// whole to its family's file for its key.
func (r *router) place() {
	var dests []destination
	var routed []bool
	k := 0
	for i := range r.units {
		u := &r.units[i]
		own, declared := familyOf(r.in.Outputs[u.Plugin], u.Tag)
		if !declared {
			k += len(u.Decls)
			r.errorf(UndeclaredFamily, r.firstAt(u), nil,
				"%s emits %d declarations into family %q, which it does not declare",
				u.Plugin, len(u.Decls), u.Tag)
			continue
		}
		home, homeFound := r.target(u.Plugin, own, u.Key, u.Pkg)
		if !r.overridden(k, len(u.Decls)) {
			k += len(u.Decls)
			if !homeFound {
				for _, d := range u.Decls {
					r.sourceless(u, d)
				}
				continue
			}
			if len(u.Decls) > 0 {
				r.add(home, i, u.Decls, true)
			}
			continue
		}
		dests, routed = dests[:0], routed[:0]
		for _, d := range u.Decls {
			ov, overridden := r.overrides[k]
			k++
			if overridden {
				dest, found := r.destine(u, d, own, ov)
				dests, routed = append(dests, dest), append(routed, found)
				continue
			}
			if !homeFound {
				r.sourceless(u, d)
			}
			dests, routed = append(dests, home), append(routed, homeFound)
		}
		r.gather(i, dests, routed)
	}
}

// overridden reports whether a declaration at one of n positions from
// k in store order has an override.
func (r *router) overridden(k, n int) bool {
	if len(r.overrides) == 0 {
		return false
	}
	for j := k; j < k+n; j++ {
		if _, held := r.overrides[j]; held {
			return true
		}
	}
	return false
}

// destine resolves the destination of one declaration with an
// override: the family its tag names, the key the family's cardinality
// derives, and the directory its path names. It reports false, with
// the finding on the sink, where no destination resolves.
func (r *router) destine(u *plugin.Unit, d symbol.Symbol, own plugin.Output, ov override) (destination, bool) {
	origin, _ := emit.OriginOf(d)
	fam := own
	if u.Tag == "" && ov.tag != "" {
		moved, known := familyOf(r.in.Outputs[u.Plugin], ov.tag)
		if !known {
			r.errorf(UnknownTag, ov.tagAt, nil,
				"tag %q names no family %s declares, and %s is refused", ov.tag, u.Plugin, describe(d))
			return destination{}, false
		}
		fam = moved
	}
	key, pkg := u.Key, u.Pkg
	if fam.Per != u.Per {
		k, owner, found := r.keyOf(fam.Per, origin)
		if !found {
			r.errorf(NoDestination, r.positionOf(origin, u), nil,
				"%s moves to family %q, and no %s key derives for it without a workspace source",
				describe(d), fam.Tag, fam.Per)
			return destination{}, false
		}
		key, pkg = k, owner
	}
	if ov.path == "" || (ov.tag != "" && ov.tag != fam.Tag) {
		dest, found := r.target(u.Plugin, fam, key, pkg)
		if !found {
			r.sourceless(u, d)
		}
		return dest, found
	}
	conf := r.in.Config.resolve(Family{Plugin: u.Plugin, Tag: fam.Tag})
	dest := destination{
		plugin: u.Plugin, tag: fam.Tag, per: fam.Per, word: fam.Word,
		key: key, pkg: pkg, file: conf.file,
	}
	base, found := r.originDir(origin)
	if !found {
		base, found = r.sourceDir(fam.Per, key, pkg)
	}
	if !found {
		r.errorf(NoDestination, ov.pathAt, nil,
			"out=%s resolves against the source directory of %s, which has none", ov.path, describe(d))
		return destination{}, false
	}
	dir, file, inside := redirect(base, ov.path)
	switch {
	case !inside:
		r.errorf(EscapingPath, ov.pathAt, nil,
			"out=%s is absolute or leaves the workspace root, and %s is refused", ov.path, describe(d))
		return destination{}, false
	case file != "" && len(r.spread[placement{origin: origin, plugin: u.Plugin}]) > 1:
		r.errorf(AmbiguousOverride, ov.pathAt, nil,
			"out=%s names one file for %s, and %s emits into more than one family from %s: "+
				"add tag= to pick one", ov.path, describe(d), u.Plugin, origin)
		return destination{}, false
	}
	dest.dir = dir
	if file != "" {
		dest.file = file
	}
	return dest, true
}

// target returns the destination of a family's file for one key under
// the plan's configuration: the configured directory for a per-plan
// family, and the key's source directory otherwise, under the output
// directory where the family is centralised. It reports false where
// the key has no source directory.
func (r *router) target(p plugin.ID, fam plugin.Output, key string, pkg symbol.Identity) (destination, bool) {
	conf := r.in.Config.resolve(Family{Plugin: p, Tag: fam.Tag})
	dest := destination{
		plugin: p, tag: fam.Tag, per: fam.Per, word: fam.Word,
		key: key, pkg: pkg, file: conf.file,
	}
	if fam.Per == plugin.PerPlan {
		dest.dir = conf.dir
		return dest, true
	}
	src, found := r.sourceDir(fam.Per, key, pkg)
	if !found {
		return destination{}, false
	}
	dest.dir = src
	if conf.policy == PolicyCentralised {
		dest.dir = path.Join(conf.dir, src)
	}
	return dest, true
}

// sourceless reports a declaration whose family's file has no source
// directory to be written beside.
func (r *router) sourceless(u *plugin.Unit, d symbol.Symbol) {
	origin, _ := emit.OriginOf(d)
	r.errorf(NoDestination, r.positionOf(origin, u), nil,
		"%s has no workspace source to be written beside", describe(d))
}

// gather adds one unit's routed declarations to their buckets. A unit
// whose declarations all route to one destination joins it whole,
// without a copy.
func (r *router) gather(unit int, dests []destination, routed []bool) {
	decls := r.units[unit].Decls
	whole := len(dests) > 0
	for j := range dests {
		if !routed[j] || dests[j] != dests[0] {
			whole = false
			break
		}
	}
	if whole {
		r.add(dests[0], unit, decls, true)
		return
	}
	for j := range dests {
		if routed[j] {
			r.add(dests[j], unit, decls[j:j+1], false)
		}
	}
}

// add appends declarations of one source unit to the bucket of a
// destination, creating the bucket on first use.
func (r *router) add(dest destination, unit int, decls []symbol.Symbol, whole bool) {
	k := dest.index()
	at, reached := r.at[k]
	if !reached {
		r.at[k] = len(r.buckets)
		b := bucket{dest: dest, first: unit, whole: whole, shared: whole, decls: decls}
		if !whole {
			b.decls = slices.Clone(decls)
		}
		r.buckets = append(r.buckets, b)
		return
	}
	b := &r.buckets[at]
	b.whole = b.whole && whole
	if b.first != unit && (len(b.more) == 0 || b.more[len(b.more)-1] != unit) {
		b.more = append(b.more, unit)
	}
	if b.shared {
		b.decls, b.shared = slices.Clip(b.decls), false
	}
	b.decls = append(b.decls, decls...)
}

// name splits each bucket through the target's speller, spells each
// part's filename, and gathers the parts by path, sorted.
func (r *router) name() error {
	for bi := range r.buckets {
		b := &r.buckets[bi]
		u := plugin.Unit{
			Plugin: b.dest.plugin, Tag: b.dest.tag, Per: b.dest.per, Word: b.dest.word,
			Key: b.dest.key, Pkg: b.dest.pkg, Decls: b.decls, Origins: r.originsOf(b),
			Contributors: r.contributorsOf(b),
		}
		parts := r.in.Speller.SplitUnit(u)
		if kept := declsOf(parts); kept != len(u.Decls) {
			return fmt.Errorf("layout: the speller splits a %s unit of %d declarations into parts of %d",
				u.Word, len(u.Decls), kept)
		}
		for pi := range parts {
			name := b.dest.file
			if name == "" {
				name = r.in.Speller.FileName(parts[pi])
			}
			if !filename(name) {
				return fmt.Errorf("layout: the speller names a %s unit %q, which is not one path element",
					u.Word, name)
			}
			at := path.Join(b.dest.dir, name)
			fi, held := r.byPath[at]
			if !held {
				r.byPath[at] = len(r.files)
				// The part's own slot of the speller's slice is the file's
				// first unit, capped so a second unit copies it out.
				r.files = append(r.files, building{file: plugin.File{Path: at, Units: parts[pi : pi+1 : pi+1]}})
				continue
			}
			r.files[fi].file.Units = append(r.files[fi].file.Units, parts[pi])
		}
	}
	slices.SortFunc(r.files, func(a, b building) int { return strings.Compare(a.file.Path, b.file.Path) })
	for fi := range r.files {
		r.byPath[r.files[fi].file.Path] = fi
	}
	return nil
}

// packages names each file's package through the target's package
// half, or gives it its first unit's package where the target has
// none. A target's error leaves the file with the zero package and
// keeps the reason, which a reference into the file reports.
func (r *router) packages() {
	for fi := range r.files {
		f := &r.files[fi]
		origin := f.file.Units[0].Pkg
		if r.in.Packager == nil {
			f.file.Pkg = origin
			continue
		}
		at := plugin.Placement{Path: f.file.Path, Origin: origin, Modules: r.in.Modules}
		if r.in.Residents != nil {
			at.Residents = r.in.Residents(path.Dir(f.file.Path))
		}
		if base := r.in.Config.ImportBase; base != "" {
			at.ImportBase, at.BaseDir = base, r.in.Config.Dir
		}
		pkg, err := r.in.Packager.PackageAt(at)
		if err != nil {
			f.reason = err
			continue
		}
		f.file.Pkg = pkg
	}
}

// assemble returns the routed files: refused declarations removed, and
// emptied units and files dropped. The files keep their path order.
func (r *router) assemble() []plugin.File {
	out := make([]plugin.File, 0, len(r.files))
	for fi := range r.files {
		f := &r.files[fi]
		units := f.file.Units[:0]
		for _, u := range f.file.Units {
			if len(r.refused) > 0 && slices.ContainsFunc(u.Decls, r.isRefused) {
				// A unit can share its declarations with a source unit or a
				// sibling part, so the removal works on a copy.
				u.Decls = slices.DeleteFunc(slices.Clone(u.Decls), r.isRefused)
			}
			if len(u.Decls) > 0 {
				units = append(units, u)
			}
		}
		if len(units) == 0 {
			continue
		}
		f.file.Units = units
		out = append(out, f.file)
	}
	return out
}

// isRefused reports whether a finding refused a declaration.
func (r *router) isRefused(d symbol.Symbol) bool {
	_, refused := r.refused[d]
	return refused
}

// refuse removes every declaration of a file from the output.
func (r *router) refuse(f *building) {
	for _, u := range f.file.Units {
		for _, d := range u.Decls {
			r.refused[d] = struct{}{}
		}
	}
}

// overrideOf returns the routing an author wrote on one origin for one
// plugin. An origin without an identity has none.
func (r *router) overrideOf(origin symbol.Identity, p plugin.ID) override {
	if origin.IsZero() {
		return override{}
	}
	return overrideOf(r.in.Index.DirectivesOf(origin), p, r.in.Directives)
}

// keyOf derives the key and package a family of one cardinality takes
// from a declaration's origin: the origin's source file for a
// per-source family, its package path for a per-package family, and
// the empty key for a per-plan family. It reports false where the
// origin has no workspace source or no package. Only an override moves
// a declaration between cardinalities, so the origin is never zero.
func (r *router) keyOf(per plugin.Cardinality, origin symbol.Identity) (string, symbol.Identity, bool) {
	if per == plugin.PerPlan {
		return "", symbol.Identity{}, true
	}
	pkg, found := r.in.Index.PackageOf(origin)
	if !found {
		return "", symbol.Identity{}, false
	}
	if per == plugin.PerPackage {
		return pkg.ID.Package, pkg.ID, true
	}
	file, found := r.sourceFile(origin)
	return file, pkg.ID, found
}

// sourceDir returns the directory a family's file is written beside: a
// per-source key's directory, and a per-package key's package
// directory, the first in path order where the package's files span
// more than one. It reports false for a per-plan family and for a key
// with no workspace source.
func (r *router) sourceDir(per plugin.Cardinality, key string, pkg symbol.Identity) (string, bool) {
	switch per {
	case plugin.PerSource:
		if !workspaceFile(key) {
			return "", false
		}
		return path.Dir(key), true
	case plugin.PerPackage:
		return r.packageDir(pkg)
	default:
		return "", false
	}
}

// packageDir returns a package's source directory, the first in path
// order where its files span more than one, and caches it.
func (r *router) packageDir(pkg symbol.Identity) (string, bool) {
	if cached, read := r.dirs[pkg]; read {
		return cached.dir, cached.found
	}
	var at packageDir
	if p, found := r.in.Index.PackageOf(pkg); found {
		for _, f := range p.Files {
			if f == nil || !workspaceFile(f.Path) {
				continue
			}
			if dir := path.Dir(f.Path); !at.found || dir < at.dir {
				at = packageDir{dir: dir, found: true}
			}
		}
	}
	r.dirs[pkg] = at
	return at.dir, at.found
}

// originDir returns the directory of an origin's source file, which a
// path override resolves against.
func (r *router) originDir(origin symbol.Identity) (string, bool) {
	file, found := r.sourceFile(origin)
	if !found {
		return "", false
	}
	return path.Dir(file), true
}

// sourceFile returns the workspace file an origin is declared in. The
// callers read an origin an override was written on, which is never
// zero.
func (r *router) sourceFile(origin symbol.Identity) (string, bool) {
	s, found := r.in.Index.Lookup(origin)
	if !found {
		return "", false
	}
	file := s.Position().File
	return file, workspaceFile(file)
}

// originsOf returns a routed unit's provenance: the source unit's own
// where the bucket is one whole unit, the source units' own where it
// is several whole units, and the distinct origins of its declarations
// otherwise.
func (r *router) originsOf(b *bucket) []symbol.Identity {
	if b.whole && len(b.more) == 0 {
		return r.units[b.first].Origins
	}
	var origins []symbol.Identity
	if b.whole {
		origins = slices.Clone(r.units[b.first].Origins)
		for _, unit := range b.more {
			origins = append(origins, r.units[unit].Origins...)
		}
	} else {
		for _, d := range b.decls {
			if origin, _ := emit.OriginOf(d); !origin.IsZero() {
				origins = append(origins, origin)
			}
		}
	}
	if len(origins) == 0 {
		return nil
	}
	slices.SortFunc(origins, symbol.Identity.Compare)
	return slices.Compact(origins)
}

// contributorsOf returns the plugins that appended into slots of a
// bucket's declarations, sorted and distinct: the contributors of
// every source unit the bucket takes declarations from. A unit records
// its contributors whole, so a declaration a tag moves takes its
// unit's contributors along.
func (r *router) contributorsOf(b *bucket) []plugin.ID {
	if len(b.more) == 0 {
		return r.units[b.first].Contributors
	}
	contributors := slices.Clone(r.units[b.first].Contributors)
	for _, unit := range b.more {
		contributors = append(contributors, r.units[unit].Contributors...)
	}
	if len(contributors) == 0 {
		return nil
	}
	slices.Sort(contributors)
	return slices.Compact(contributors)
}

// positionOf returns where a finding about a declaration is
// positioned: its origin's position, and its unit's where the origin
// has none.
func (r *router) positionOf(origin symbol.Identity, u *plugin.Unit) position.Pos {
	if !origin.IsZero() {
		if s, found := r.in.Index.Lookup(origin); found {
			if pos := s.Position(); !pos.IsZero() {
				return pos
			}
		}
	}
	return unitAt(u)
}

// firstAt positions a finding about a whole unit at its first
// declaration's origin.
func (r *router) firstAt(u *plugin.Unit) position.Pos {
	if len(u.Decls) == 0 {
		return unitAt(u)
	}
	origin, _ := emit.OriginOf(u.Decls[0])
	return r.positionOf(origin, u)
}

// errorf reports one routing finding at Error severity under the
// layout phase.
func (r *router) errorf(c diag.Code, at position.Pos, related []position.Pos, format string, a ...any) {
	r.in.Sink.Report(diag.Diag{
		Code:     c,
		Severity: diag.SeverityError,
		Pos:      at,
		Msg:      fmt.Sprintf(format, a...),
		Origin:   diag.PhaseLayout,
		Related:  related,
	})
}

// familyOf returns the declared family of one tag.
func familyOf(outputs []plugin.Output, tag string) (plugin.Output, bool) {
	for _, o := range outputs {
		if o.Tag == tag {
			return o, true
		}
	}
	return plugin.Output{}, false
}

// declsOf counts the declarations of a speller's parts.
func declsOf(parts []plugin.Unit) int {
	n := 0
	for _, p := range parts {
		n += len(p.Decls)
	}
	return n
}

// workspaceFile reports whether a path names a file of the workspace
// tree, and not a file of a dependency store.
func workspaceFile(p string) bool {
	if p == "" {
		return false
	}
	_, _, qualified := plugin.CutStorePath(p)
	return !qualified
}

// unitAt positions a finding about a unit with no positioned origin:
// its routing key, or its plugin's name for a plan unit, which derives
// from no source.
func unitAt(u *plugin.Unit) position.Pos {
	if u.Key != "" {
		return position.Pos{File: u.Key}
	}
	return position.Pos{File: string(u.Plugin)}
}

// describe names a declaration in a finding: its kind and its name,
// or its kind alone for a declaration without a name.
func describe(d symbol.Symbol) string {
	name := emit.DeclaredName(d)
	if m, method := d.(*emit.Method); method {
		name = m.Name
	}
	if name == "" {
		return d.Kind().String()
	}
	return d.Kind().String() + " " + name
}
