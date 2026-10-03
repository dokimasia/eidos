// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"cmp"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// evaluatedMatch is one match a candidate has: the candidate's index
// into the selection's candidates, and the match's key.
type evaluatedMatch struct {
	candidate int
	key       plugin.MatchKey
}

// selection is a phase call's view of its [plugin.Selection]: the
// listed matches in canonical match order, the candidates in identity
// order, and the matches each candidate has under the rules the call ran
// so far. A repeated key or candidate follows the one it repeats, and
// the call skips it. The call reads the view and appends to evaluated on
// the calling goroutine.
//
// # Allocation contract
//
// A selection whose keys and candidates arrive in their contract's order
// is read in place, and binding it allocates nothing. One out of order
// costs a sorted copy.
type selection struct {
	matches    []plugin.MatchKey
	candidates []symbol.Identity
	// evaluated contains each match a candidate has, with the candidate's
	// index into candidates, in the order the rules found them.
	evaluated []evaluatedMatch
}

// bind makes s the view of sel: its keys and its candidates as given
// where they are in canonical match order and identity order, and a
// sorted copy of either where it is not, so the call's matches do not
// depend on the caller keeping that order.
func (s *selection) bind(sel *plugin.Selection) {
	s.matches, s.candidates = sel.Matches, sel.Candidates
	if !slices.IsSortedFunc(s.matches, plugin.MatchKey.Compare) {
		s.matches = slices.SortedFunc(slices.Values(sel.Matches), plugin.MatchKey.Compare)
	}
	if !slices.IsSortedFunc(s.candidates, symbol.Identity.Compare) {
		s.candidates = slices.SortedFunc(slices.Values(sel.Candidates), symbol.Identity.Compare)
	}
}

// listed returns the keys the selection lists for one rule of a plugin,
// in subject then instance order: a range of the sorted keys, which no
// key of another plugin or another rule enters.
func (s *selection) listed(id plugin.ID, rule plugin.RuleID) []plugin.MatchKey {
	lo, _ := slices.BinarySearchFunc(s.matches, plugin.MatchKey{Plugin: id, Rule: rule}, byRule)
	n, _ := slices.BinarySearchFunc(s.matches[lo:], plugin.MatchKey{Plugin: id, Rule: rule + 1}, byRule)
	return s.matches[lo : lo+n]
}

// evaluatedOf returns the keys of the matches the j-th candidate has, in
// the order the rules found them, appended to dst[:0]. The delivery
// sorts evaluated by candidate first, and from is where the candidate's
// keys start, which it returns advanced past them.
func (s *selection) evaluatedOf(j, from int, dst []plugin.MatchKey) ([]plugin.MatchKey, int) {
	dst = dst[:0]
	for ; from < len(s.evaluated) && s.evaluated[from].candidate == j; from++ {
		dst = append(dst, s.evaluated[from].key)
	}
	return dst, from
}

// byRule orders a key against a target by plugin, then rule: the prefix
// of canonical match order that groups one rule's keys.
func byRule(k, target plugin.MatchKey) int {
	return cmp.Or(strings.Compare(string(k.Plugin), string(target.Plugin)), cmp.Compare(k.Rule, target.Rule))
}

// bySubjectInstance orders two keys of one rule: by subject, then by
// gating instance. A rule that is not emit-phase has no host, so the
// two fields name its match.
func bySubjectInstance(a, b plugin.MatchKey) int {
	return cmp.Or(a.Subject.Compare(b.Subject), cmp.Compare(a.Instance, b.Instance))
}

// restrict binds the call to a selection, and leaves a call without one
// running every match.
func (c *phaseCall) restrict(sel *plugin.Selection) {
	if sel == nil {
		return
	}
	c.selects = true
	c.selection.bind(sel)
}

// enumerateSelected visits the matches the selection admits under one
// rule that is not emit-phase, in canonical match order: a graph-wide
// rule where the selection lists it, and otherwise each subject that
// the rule's listed matches or the candidates name, in identity order.
// A listed match runs where the rule's gates still admit it, and a
// candidate runs every match the gates admit, its listed matches
// included. A repeat runs nothing. It stops where take reports false.
func (c *phaseCall) enumerateSelected(fr *flatRule) {
	listed := c.selection.listed(c.plugin, fr.ordinal)
	if fr.graph {
		if len(listed) > 0 {
			c.take(fr, &invocation{})
		}
		return
	}
	candidates := c.selection.candidates
	i, j := 0, 0
	for i < len(listed) || j < len(candidates) {
		switch {
		case i > 0 && i < len(listed) && bySubjectInstance(listed[i], listed[i-1]) == 0:
			i++
		case j > 0 && j < len(candidates) && candidates[j] == candidates[j-1]:
			j++
		case j == len(candidates) || (i < len(listed) && listed[i].Subject.Compare(candidates[j]) < 0):
			if !c.takeListed(fr, listed[i]) {
				return
			}
			i++
		default:
			for i < len(listed) && listed[i].Subject == candidates[j] {
				i++
			}
			if !c.takeCandidate(fr, j) {
				return
			}
			j++
		}
	}
}

// takeListed hands take one listed match where the rule's gates still
// admit it: none where the subject is gone or refused, and where a
// directive gates the rule, none where the subject has the listed
// instance no longer. A rule no directive gates has one match per
// subject, at instance zero. It reports false where take stopped the
// rule.
func (c *phaseCall) takeListed(fr *flatRule, k plugin.MatchKey) bool {
	s, held := c.admitted(fr, k.Subject)
	if !held {
		return true
	}
	inv := invocation{subject: k.Subject, pos: s.Position(), value: s}
	if fr.gate == "" {
		if k.Instance != 0 {
			return true
		}
		return c.take(fr, &inv)
	}
	ds := c.index.DirectivesOf(k.Subject)
	for i := range ds {
		if gates(fr, &ds[i]) && ds[i].Instance == k.Instance {
			inv.gate = &ds[i]
			return c.take(fr, &inv)
		}
	}
	return true
}

// takeCandidate evaluates the rule's gates over the j-th candidate,
// records the matches it has under the rule, and hands them to take as
// a full enumeration does. It reports false where take stopped the
// rule.
func (c *phaseCall) takeCandidate(fr *flatRule, j int) bool {
	id := c.selection.candidates[j]
	s, held := c.admitted(fr, id)
	if !held {
		return true
	}
	key := plugin.MatchKey{Plugin: c.plugin, Rule: fr.ordinal, Subject: id}
	if fr.gate == "" {
		c.selection.evaluated = append(c.selection.evaluated, evaluatedMatch{candidate: j, key: key})
	} else {
		ds := c.index.DirectivesOf(id)
		for i := range ds {
			if gates(fr, &ds[i]) {
				key.Instance = ds[i].Instance
				c.selection.evaluated = append(c.selection.evaluated, evaluatedMatch{candidate: j, key: key})
			}
		}
	}
	return c.takeEach(fr, &invocation{subject: id, pos: s.Position(), value: s})
}

// admitted returns the declaration a rule's gates admit under one
// identity: one the scope admits, of the rule's kind, that the rule's
// skip and predicates admit. It reports false for any other identity. A
// full enumeration admits the same declarations through the rule's
// index.
func (c *phaseCall) admitted(fr *flatRule, id symbol.Identity) (symbol.Symbol, bool) {
	s, held := c.index.Lookup(id)
	if !held || s.Kind() != fr.kind {
		return nil, false
	}
	if _, names := s.(node.Declaration); !names || !c.admits(fr, id) {
		return nil, false
	}
	return s, true
}
