// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"path"
	"slices"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// touch is what one invocation of a plan's generator touched: its match,
// and the units it placed declarations into or appended into the slots
// of, each once. units is a window of its journal's buffer whose capacity
// ends at its length, so an append to it copies.
type touch struct {
	match plugin.MatchKey
	units []plugin.UnitRef
}

// touchJournal is the journal of a recording plan's generator calls. It
// keeps what each invocation touched, which the plan's groups read, and
// passes each record on to next. The units of every touch share one
// buffer, which grows by doubling.
//
// # Concurrency
//
// A touchJournal is not safe for concurrent use. A phase call journals on
// the goroutine that made the call, and the plan's calls run one at a time.
type touchJournal struct {
	touches []touch
	units   []plugin.UnitRef
	next    plugin.Journal
}

var _ plugin.Journal = (*touchJournal)(nil)

// Invoked keeps the units that the invocation placed into and the units
// of the values whose slots it appended into, and passes the record on.
func (j *touchJournal) Invoked(inv plugin.Invocation) {
	start := len(j.units)
	j.units = append(j.units, inv.Units...)
	for _, h := range inv.Hosts {
		j.units = append(j.units, h.Unit)
	}
	units := j.units[start:]
	slices.SortFunc(units, plugin.UnitRef.Compare)
	units = slices.Compact(units)
	j.units = j.units[:start+len(units)]
	j.touches = append(j.touches, touch{match: inv.Match, units: slices.Clip(units)})
	j.next.Invoked(inv)
}

// Evaluated passes the matches of a candidate on.
func (j *touchJournal) Evaluated(subject symbol.Identity, matches []plugin.MatchKey) {
	j.next.Evaluated(subject, matches)
}

// planGroups is the input of one plan's groups: the plan's settled store,
// the files its layout routed and its render rendered, in path order,
// with their staged forms at the same places, what each invocation
// touched, what the settle read, and the names of the plan's files that a
// warm run keeps, nil for a store that contains the whole plan. placed
// reports that the backend derives each file's package from the
// residents of its directory and from the modules.
type planGroups struct {
	plan    string
	emit    *plugin.Emit
	files   []plugin.File
	staged  []stagedFile
	touches []touch
	settled plugin.Settled
	others  plugin.Names
	placed  bool
}

// record records the plan's groups into lane, and sets the group and the
// names of each staged file.
//
// A group is the set of units that execute together, and the files that
// contain their declarations: two units are in one group when a file
// contains declarations of both, and when an invocation of an emit-phase
// rule matched a value of one and placed into or appended into the other.
// A group's key is its first unit in [plugin.UnitRef.Compare] order. Its
// contributors are the invocations that touched one of its units. Its
// reads are the edges that the group reads beyond its invocations:
//
//   - the edge of each of its units, which an invocation that places into
//     the unit makes dirty
//   - the edge of each name that the settle looked up for one of its units,
//     and of each override that the settle read for one
//   - the scope of each name that its files declare
//   - the declaration edge of each origin of its declarations, whose
//     directives and position the layout reads, and the files edge of the
//     package of each per-package unit, whose directory the layout reads
//   - the residents of the directory of each of its files and the modules,
//     where the backend places its files against them
//
// Its findings are those that the render reported for its files.
func (pg *planGroups) record(lane *state.Lane) {
	units := slices.Collect(pg.emit.Units())
	at := make(map[plugin.UnitRef]int, len(units))
	owner := map[symbol.Symbol]int{}
	for i, u := range units {
		at[u.Ref()] = i
		for _, d := range u.Decls {
			owner[d] = i
		}
	}
	parent := make([]int, len(units))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	join := func(a, b int) {
		if ra, rb := find(a), find(b); ra != rb {
			parent[max(ra, rb)] = min(ra, rb)
		}
	}
	for _, f := range pg.files {
		first := -1
		for _, u := range f.Units {
			for _, d := range u.Decls {
				i, held := owner[d]
				switch {
				case !held:
				case first < 0:
					first = i
				default:
					join(first, i)
				}
			}
		}
	}
	for _, t := range pg.touches {
		host, held := at[t.match.Host.Unit]
		if t.match.Host == (plugin.EmitRef{}) || !held {
			continue
		}
		for _, u := range t.units {
			if i, placed := at[u]; placed {
				join(host, i)
			}
		}
	}

	groups := map[int]*state.Group{}
	group := func(i int) *state.Group {
		root := find(i)
		g := groups[root]
		if g == nil {
			g = &state.Group{}
			groups[root] = g
		}
		return g
	}
	for i, u := range units {
		g := group(i)
		g.Units = append(g.Units, u.Ref())
		g.Reads = append(g.Reads, state.UnitEdge(pg.plan, u.Ref()))
		if u.Per == plugin.PerPackage {
			g.Reads = append(g.Reads, state.FilesEdge(u.Pkg))
		}
		for _, d := range u.Decls {
			if origin, _ := emit.OriginOf(d); !origin.IsZero() {
				g.Reads = append(g.Reads, state.DeclarationEdge(origin))
			}
		}
	}
	fileGroup := make([]int, len(pg.files))
	for fi, f := range pg.files {
		fileGroup[fi] = -1
		if len(f.Units) == 0 || len(f.Units[0].Decls) == 0 {
			continue
		}
		i, held := owner[f.Units[0].Decls[0]]
		if !held {
			continue
		}
		fileGroup[fi] = find(i)
		g := group(i)
		g.Files = append(g.Files, f.Path)
		if pg.placed {
			g.Reads = append(g.Reads, state.DirectoryEdge(path.Dir(f.Path)), state.ModulesEdge)
		}
	}
	// hit lists the groups that one invocation touched, each once. An
	// invocation touches few units, so a scan of the list finds a repeat.
	// A first pass counts each group's contributors, and the second fills
	// each group's window of one list.
	var hit []int
	mark := func(u plugin.UnitRef) {
		if i, held := at[u]; held {
			if root := find(i); !slices.Contains(hit, root) {
				hit = append(hit, root)
			}
		}
	}
	touched := func(t touch) []int {
		hit = hit[:0]
		mark(t.match.Host.Unit)
		for _, u := range t.units {
			mark(u)
		}
		return hit
	}
	counts := make([]int, len(units))
	total := 0
	for _, t := range pg.touches {
		for _, root := range touched(t) {
			counts[root]++
			total++
		}
	}
	contributors := make([]plugin.MatchKey, total)
	for root, n := range counts {
		if n > 0 {
			groups[root].Contributors, contributors = contributors[:0:n], contributors[n:]
		}
	}
	for _, t := range pg.touches {
		for _, root := range touched(t) {
			g := groups[root]
			g.Contributors = append(g.Contributors, t.match)
		}
	}
	for _, r := range pg.settled.Read {
		if r.Unit < len(units) {
			g := group(r.Unit)
			g.Reads = append(g.Reads, state.NameEdge(pg.plan, r.Key))
		}
	}
	for _, r := range pg.settled.Facts {
		if r.Unit < len(units) {
			g := group(r.Unit)
			g.Reads = append(g.Reads, state.FactEdge(r.Fact.Subject, r.Fact.Key))
		}
	}
	names := plugin.NamesOf(pg.files, pg.emit, pg.others)
	for fi := range pg.staged {
		s := &pg.staged[fi]
		count := 0
		for count < len(names) && names[count].File == s.path {
			count++
		}
		if count > 0 {
			s.names, names = names[:count:count], names[count:]
		}
		if fi >= len(fileGroup) || fileGroup[fi] < 0 {
			continue
		}
		g := groups[fileGroup[fi]]
		for _, n := range s.names {
			g.Reads = append(g.Reads, state.ScopeEdge(pg.plan, n.Package, n.Receiver))
		}
		g.Findings = append(g.Findings, s.findings...)
	}

	roots := make([]int, 0, len(groups))
	for root := range groups {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	for _, root := range roots {
		g := groups[root]
		slices.SortFunc(g.Units, plugin.UnitRef.Compare)
		g.Key = g.Units[0]
		slices.SortFunc(g.Contributors, plugin.MatchKey.Compare)
		g.Contributors = slices.CompactFunc(g.Contributors, func(a, b plugin.MatchKey) bool {
			return a.Compare(b) == 0
		})
		lane.Group(*g)
	}
	for fi := range pg.staged {
		if fi < len(fileGroup) && fileGroup[fi] >= 0 {
			pg.staged[fi].group = groups[fileGroup[fi]].Key
		}
	}
}
