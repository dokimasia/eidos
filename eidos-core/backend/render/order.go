// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"cmp"
	"slices"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
)

// declRef is the position of one declaration of a file: the index of its
// unit among the units of the file, and its index among the declarations
// of the unit.
type declRef struct {
	unit, decl int
}

// edge is one constraint of the order of a file: the block before comes
// before the block after, because a declaration of after evaluates the
// name of before.
type edge struct {
	before, after int
}

// orderScratch is the memory that the order of a file reuses from one file
// to the next of a frame, so a worker grows it once for its largest file.
// The order that [frame.order] returns is out, which the order of the next
// file overwrites.
//
// A block is one entry of the order: the declarations of one function
// name, the marked signatures first, or one other declaration. The blocks
// are numbered in the order of the flush of their first declarations, so
// a lower number comes first between two blocks that can both come next.
type orderScratch struct {
	flush     []declRef      // the declarations of the file in the order of the flush
	blockOf   []int          // the block of each position of flush
	marked    []bool         // whether each position of flush is a marked function
	kept      []bool         // whether each position of flush renders
	byName    map[string]int // the block of each declared name of the file
	functions []bool         // whether each block gathers the functions of a name
	starts    []int          // where the members of each block start in members
	cursor    []int          // where the next member of each block goes in members
	members   []int          // the positions of the members of every block, block by block
	edges     []edge         // the constraints, sorted by the block before
	outgoing  []int          // where the edges of each block before start in edges
	indegree  []int          // the constraints on each block that its blocks before have not met
	placed    []bool         // whether each block is in out
	ready     []int          // the blocks without an unmet constraint, the lowest last
	runs      []clusterRun   // the clusters of each unit, for the render in the order
	out       []declRef      // the order of the file
	current   int            // the block whose declaration yields names to [orderScratch.evaluated]
	yield     func(string)   // the method value of [orderScratch.evaluated], bound once
}

// order returns the declarations of the file g in the order of its
// language:
//
//   - The functions of one declared name form one block at the position of
//     the first of them, with the marked signatures first in the order of
//     the flush and the other functions of the name after them. Every
//     other declaration is a block of its own.
//   - Where the language states [Language.Evaluated], a block follows each
//     block of a name that one of its declarations evaluates. Among the
//     blocks that can come next, the block at the lowest position of the
//     flush comes first. A cycle keeps the order of the flush for the
//     block at its lowest position, and the backend refuses the reference
//     that comes too early.
//   - Where the language is [Language.Implemented], the order leaves out a
//     block of overload signatures without an implementation, and each
//     implementation after the first of a block, and reports each under
//     [UnimplementedOverload].
//
// order reports false for a language that states neither, so the file
// keeps the order of the flush. The returned order is the frame's scratch,
// which the next file overwrites. A frame grows its scratch for its largest
// file and allocates nothing after that, apart from the findings that it
// reports.
func (f *frame) order(g *plugin.File) ([]declRef, bool) {
	if f.pass.evaluated == nil && !f.pass.implemented {
		return nil, false
	}
	s := &f.ord
	size := 0
	for _, u := range g.Units {
		size += len(u.Decls)
	}
	s.flush = slices.Grow(s.flush[:0], size)
	for ui, u := range g.Units {
		for di, d := range u.Decls {
			if _, header := d.(*emit.File); !header {
				s.flush = append(s.flush, declRef{unit: ui, decl: di})
			}
		}
	}
	blocks := s.block(g)
	s.group(blocks)
	if f.pass.implemented {
		f.check(g, blocks)
	}
	s.edges = s.edges[:0]
	if f.pass.evaluated != nil {
		s.constrain(g, blocks, f.pass.evaluated)
	}
	s.sort(blocks)
	return s.out, true
}

// reserve takes the names of the declarations of the file g for the import
// set of the frame in the order in which they render, so a language that
// reads whether a name is pending reads the order of the file. That order
// is the order of the flush where the file is not ordered, and order
// otherwise, followed by the declarations that the order leaves out. It
// returns the number of the declarations of the file.
func (f *frame) reserve(g *plugin.File, order []declRef, ordered bool) int {
	if !ordered {
		declared := 0
		for _, u := range g.Units {
			for _, d := range u.Decls {
				if _, header := d.(*emit.File); header {
					continue
				}
				declared++
				f.set.Reserve(emit.DeclaredName(d))
			}
		}
		return declared
	}
	s := &f.ord
	for _, r := range order {
		f.set.Reserve(emit.DeclaredName(g.Units[r.unit].Decls[r.decl]))
	}
	for at, r := range s.flush {
		if !s.kept[at] {
			f.set.Reserve(emit.DeclaredName(g.Units[r.unit].Decls[r.decl]))
		}
	}
	return len(s.flush)
}

// block numbers the blocks of the declarations of flush, records the
// block of each declared name, marks the signatures of overloaded
// functions and keeps every declaration. It returns the number of blocks.
func (s *orderScratch) block(g *plugin.File) int {
	n := len(s.flush)
	s.blockOf = slices.Grow(s.blockOf[:0], n)[:n]
	s.marked = slices.Grow(s.marked[:0], n)[:n]
	s.kept = slices.Grow(s.kept[:0], n)[:n]
	s.functions = slices.Grow(s.functions[:0], n)
	if s.byName == nil {
		s.byName = make(map[string]int, n)
	}
	clear(s.byName)
	for at, r := range s.flush {
		d := g.Units[r.unit].Decls[r.decl]
		name := emit.DeclaredName(d)
		fn, function := d.(*emit.Function)
		s.marked[at] = function && fn != nil && fn.Overload
		s.kept[at] = true
		b, held := s.byName[name]
		if held && function && s.functions[b] {
			s.blockOf[at] = b
			continue
		}
		s.blockOf[at] = len(s.functions)
		if !held && name != "" {
			s.byName[name] = len(s.functions)
		}
		s.functions = append(s.functions, function)
	}
	return len(s.functions)
}

// group lists the members of every block in members, block by block, in
// the order of the flush, and puts the marked signatures of a block of
// functions before its other functions.
func (s *orderScratch) group(blocks int) {
	s.starts = slices.Grow(s.starts[:0], blocks+1)[:blocks+1]
	clear(s.starts)
	for _, b := range s.blockOf {
		s.starts[b+1]++
	}
	for b := range blocks {
		s.starts[b+1] += s.starts[b]
	}
	s.cursor = append(s.cursor[:0], s.starts[:blocks]...)
	s.members = slices.Grow(s.members[:0], len(s.flush))[:len(s.flush)]
	for at, b := range s.blockOf {
		s.members[s.cursor[b]] = at
		s.cursor[b]++
	}
	for b := range blocks {
		if ms := s.members[s.starts[b]:s.starts[b+1]]; s.functions[b] && len(ms) > 1 {
			slices.SortStableFunc(ms, s.signaturesFirst)
		}
	}
}

// signaturesFirst orders a marked signature before a function without the
// mark, and keeps the order of two of one kind.
func (s *orderScratch) signaturesFirst(a, b int) int {
	switch {
	case s.marked[a] == s.marked[b]:
		return 0
	case s.marked[a]:
		return -1
	default:
		return 1
	}
}

// check leaves out of the order a block of overload signatures without an
// implementation, and each implementation after the first of a block, and
// reports each under [UnimplementedOverload].
func (f *frame) check(g *plugin.File, blocks int) {
	s := &f.ord
	for b := range blocks {
		ms := s.members[s.starts[b]:s.starts[b+1]]
		if !s.functions[b] {
			continue
		}
		signatures := 0
		for _, at := range ms {
			if s.marked[at] {
				signatures++
			}
		}
		first := s.flush[ms[0]]
		name := emit.DeclaredName(g.Units[first.unit].Decls[first.decl])
		switch {
		case signatures > 0 && signatures == len(ms):
			for _, at := range ms {
				s.kept[at] = false
			}
			f.sink.Errorf(UnimplementedOverload, f.at, f.origin,
				"the overload signatures of %s in %s have no implementation, and they are skipped", name, g.Path)
		case len(ms)-signatures > 1:
			for _, at := range ms[signatures+1:] {
				s.kept[at] = false
				f.sink.Errorf(UnimplementedOverload, f.at, f.origin,
					"%s has a second implementation in %s, and it is skipped", name, g.Path)
			}
		}
	}
}

// constrain records an edge from each block of a name that a kept
// declaration evaluates to the block of the declaration, sorts the edges
// by the block before, and records where the edges of each block start.
func (s *orderScratch) constrain(g *plugin.File, blocks int, evaluated Evaluated) {
	if s.yield == nil {
		s.yield = s.evaluated
	}
	for at, r := range s.flush {
		if !s.kept[at] {
			continue
		}
		s.current = s.blockOf[at]
		evaluated(g.Units[r.unit].Decls[r.decl], s.yield)
	}
	slices.SortFunc(s.edges, func(a, b edge) int { return cmp.Compare(a.before, b.before) })
	s.outgoing = slices.Grow(s.outgoing[:0], blocks+1)[:blocks+1]
	clear(s.outgoing)
	for _, e := range s.edges {
		s.outgoing[e.before+1]++
	}
	for b := range blocks {
		s.outgoing[b+1] += s.outgoing[b]
	}
}

// evaluated records an edge from the block of name to the current block,
// where a declaration of the file declares name and the declaration is not
// in the current block.
func (s *orderScratch) evaluated(name string) {
	if before, held := s.byName[name]; held && before != s.current {
		s.edges = append(s.edges, edge{before: before, after: s.current})
	}
}

// sort writes the kept declarations of every block into out, a block after
// each block before it, and the lowest of the blocks that can come next
// first. A cycle places the lowest block that is not placed, and its
// constraints count as met.
func (s *orderScratch) sort(blocks int) {
	s.indegree = slices.Grow(s.indegree[:0], blocks)[:blocks]
	clear(s.indegree)
	for _, e := range s.edges {
		s.indegree[e.after]++
	}
	s.placed = slices.Grow(s.placed[:0], blocks)[:blocks]
	clear(s.placed)
	s.ready = slices.Grow(s.ready[:0], blocks)
	for b := blocks - 1; b >= 0; b-- {
		if s.indegree[b] == 0 {
			s.ready = append(s.ready, b)
		}
	}
	s.out = slices.Grow(s.out[:0], len(s.flush))
	for range blocks {
		var b int
		if len(s.ready) > 0 {
			b = s.ready[len(s.ready)-1]
			s.ready = s.ready[:len(s.ready)-1]
		} else {
			b = slices.Index(s.placed, false)
		}
		s.placed[b] = true
		for _, at := range s.members[s.starts[b]:s.starts[b+1]] {
			if s.kept[at] {
				s.out = append(s.out, s.flush[at])
			}
		}
		if len(s.edges) == 0 {
			continue
		}
		for _, e := range s.edges[s.outgoing[b]:s.outgoing[b+1]] {
			s.indegree[e.after]--
			if s.indegree[e.after] != 0 || s.placed[e.after] {
				continue
			}
			i, _ := slices.BinarySearchFunc(s.ready, e.after, func(r, after int) int { return cmp.Compare(after, r) })
			s.ready = slices.Insert(s.ready, i, e.after)
		}
	}
}
