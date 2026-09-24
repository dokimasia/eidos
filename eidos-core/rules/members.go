// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// MemberSet is a type's effective members: its own and what arrives
// through its contributors, with provenance per member and a gap
// for every contributor that yielded nothing.
type MemberSet struct {
	Members []Member
	Gaps    []Gap
}

// Complete reports whether every contributor yielded.
func (s MemberSet) Complete() bool { return len(s.Gaps) == 0 }

// Member is one effective member and where it came from.
type Member struct {
	// Symbol is the member: a field, a method, or, under
	// [MemberPolicy.EmbedsAreFields], the [node.Embed] declaring an
	// embedded field. One that arrived through a contributor carrying
	// type arguments is a copy with the arguments bound; a declared
	// one is the graph's own node.
	Symbol symbol.Symbol
	// Owner is the declaration that declared it.
	Owner symbol.Identity
	// Through lists the contributors traversed, outermost first;
	// empty for a declared member.
	Through []symbol.Identity
	// Depth is how many contributors were traversed.
	Depth int
	// ViaOptional records that a contributor on the path has the
	// Optional form: a Go pointer embed promotes the pointer
	// receiver's methods as well as the value receiver's.
	ViaOptional bool
}

// GapReason says why a contributor yielded nothing.
type GapReason uint8

const (
	// GapUnresolved is a contributor whose reference carries no
	// target, or one the view does not hold.
	GapUnresolved GapReason = iota + 1
	// GapNotMembered is a contributor that is not a type with
	// members.
	GapNotMembered
	// GapCyclic is a contributor already on the path, or one past
	// the depth budget.
	GapCyclic
	// GapGeneric is a contributor carrying type arguments the walk
	// could not bind.
	GapGeneric
	// GapConflict is two arrivals of one name on an interface with
	// different signatures.
	GapConflict
)

// String returns the reason's spelling.
func (r GapReason) String() string {
	switch r {
	case GapUnresolved:
		return "unresolved"
	case GapNotMembered:
		return "not-membered"
	case GapCyclic:
		return "cyclic"
	case GapGeneric:
		return "generic"
	case GapConflict:
		return "conflict"
	default:
		return strconv.Itoa(int(r))
	}
}

// Gap is one contributor that yielded nothing, and why.
type Gap struct {
	Host        symbol.Identity
	Contributor *node.TypeRef
	Reason      GapReason
}

// membered is what the walk reads off a type with members.
type membered struct {
	id         symbol.Identity
	fields     []*node.Field
	methods    []*node.Method
	embeds     []*node.Embed
	extends    []*node.TypeRef
	implements []*node.TypeRef
	params     []*node.TypeParam
	iface      bool
}

// memberedOf reads the lists a kind carries, and false for a kind
// without members.
func memberedOf(sym symbol.Symbol) (membered, bool) {
	switch d := sym.(type) {
	case *node.Struct:
		if d == nil {
			return membered{}, false
		}
		return membered{
			id: d.ID, fields: d.Fields, methods: d.Methods, embeds: d.Embeds,
			extends: d.Extends, implements: d.Implements, params: d.TypeParams,
		}, true
	case *node.Interface:
		if d == nil {
			return membered{}, false
		}
		return membered{
			id: d.ID, fields: d.Fields, methods: d.Methods, embeds: d.Embeds,
			extends: d.Extends, params: d.TypeParams, iface: true,
		}, true
	case *node.Enum:
		if d == nil {
			return membered{}, false
		}
		return membered{id: d.ID, fields: d.Fields, methods: d.Methods}, true
	case *node.Sum:
		if d == nil {
			return membered{}, false
		}
		return membered{id: d.ID, methods: d.Methods, params: d.TypeParams}, true
	default:
		return membered{}, false
	}
}

// arrival is one candidate for a name, chained to the next arrival
// of the same name by index so one flat list holds every name's
// candidates without a slice per name.
type arrival struct {
	member Member
	// via is the contributor the member arrived by, nil for a
	// declared member: what a conflict gap names.
	via *node.TypeRef
	// sig is the member's signature, spelled the first time a rule
	// compares one. signed reports whether it is spelled.
	sig     string
	signed  bool
	next    int  // index of the next arrival of this name, or -1
	dropped bool // folded away by an interface's set semantics
}

// signature returns the arrival's signature, spelled on first use:
// a method's, and empty for a field or an embedded field.
func (a *arrival) signature() string {
	if !a.signed {
		if m, is := a.member.Symbol.(*node.Method); is {
			a.sig = signature(m)
		}
		a.signed = true
	}
	return a.sig
}

// chain is where a name's arrivals start and currently end.
type chain struct{ first, last int }

// walk carries one member walk's state.
type walk struct {
	b        Bound
	root     symbol.Identity
	arrivals []arrival
	names    map[string]chain
	order    []string
	gaps     []Gap
	visiting map[symbol.Identity]bool
}

// membersOf runs the kernel walk under the subject language's
// policy. It reports false for a symbol that is not a type.
func (b Bound) membersOf(sym symbol.Symbol) (MemberSet, bool) {
	root, is := memberedOf(sym)
	if !is {
		return MemberSet{}, false
	}
	declared := len(root.fields) + len(root.methods)
	w := &walk{
		b:        b,
		root:     root.id,
		arrivals: make([]arrival, 0, declared),
		names:    make(map[string]chain, declared),
		order:    make([]string, 0, declared),
		visiting: map[symbol.Identity]bool{},
	}
	policy := b.source.Members()
	w.visit(root, policy, b.source, nil, 0, false, nil)
	set := MemberSet{Members: make([]Member, 0, len(w.order))}
	for _, name := range w.order {
		set.Members = w.settle(set.Members, name, policy, root.iface)
	}
	// Settling files conflict gaps of its own, so the gaps are read
	// after it, not before.
	set.Gaps = w.gaps
	return set, true
}

// visit records a type's declared members at the current depth and
// descends into its contributors under the policy that applies to
// it. The members record in list order: fields, embedded fields
// where the policy records them, then methods. via is the
// contributor that reached the type, nil at the root.
func (w *walk) visit(
	t membered, policy MemberPolicy, source SourceRules,
	through []symbol.Identity, depth int, viaOptional bool, via *node.TypeRef,
) {
	if !t.id.IsZero() {
		w.visiting[t.id] = true
		defer delete(w.visiting, t.id)
	}
	for _, f := range t.fields {
		if f != nil && f.Name != "" {
			m := Member{Symbol: f, Owner: t.id, Through: through, Depth: depth, ViaOptional: viaOptional}
			w.record(f.Name, m, via)
		}
	}
	if policy.EmbedsAreFields && !t.iface {
		for _, e := range t.embeds {
			if e != nil && e.ID.Name != "" {
				m := Member{Symbol: e, Owner: t.id, Through: through, Depth: depth, ViaOptional: viaOptional}
				w.record(e.ID.Name, m, via)
			}
		}
	}
	for _, m := range t.methods {
		if m != nil && m.Name != "" {
			mem := Member{Symbol: m, Owner: t.id, Through: through, Depth: depth, ViaOptional: viaOptional}
			w.record(m.Name, mem, via)
		}
	}
	budget := policy.Depth
	if budget <= 0 {
		budget = DefaultDepth
	}
	for _, c := range policy.Contributes {
		for _, ref := range t.contributors(c) {
			w.descend(t, ref, source, through, depth, budget, viaOptional)
		}
	}
}

// contributors returns one list's references.
func (t membered) contributors(c Contribution) []*node.TypeRef {
	switch c {
	case ContributesEmbeds:
		out := make([]*node.TypeRef, 0, len(t.embeds))
		for _, e := range t.embeds {
			if e != nil {
				out = append(out, e.Ref)
			}
		}
		return out
	case ContributesExtends:
		return t.extends
	case ContributesImplements:
		return t.implements
	default:
		return nil
	}
}

// descend follows one contributor, recording a gap where it cannot.
func (w *walk) descend(
	host membered, ref *node.TypeRef, source SourceRules,
	through []symbol.Identity, depth, budget int, viaOptional bool,
) {
	if ref == nil {
		return
	}
	target := namedUnder(ref)
	if target == nil || target.Target.IsZero() {
		w.gaps = append(w.gaps, Gap{Host: host.id, Contributor: ref, Reason: GapUnresolved})
		return
	}
	if w.visiting[target.Target] || depth+1 > budget {
		w.gaps = append(w.gaps, Gap{Host: host.id, Contributor: ref, Reason: GapCyclic})
		return
	}
	decl, held := w.b.view.Lookup(target.Target)
	if !held {
		w.gaps = append(w.gaps, Gap{Host: host.id, Contributor: ref, Reason: GapUnresolved})
		return
	}
	inner, is := memberedOf(decl)
	if !is {
		w.gaps = append(w.gaps, Gap{Host: host.id, Contributor: ref, Reason: GapNotMembered})
		return
	}
	if len(target.Args) > 0 || len(inner.params) > 0 {
		bound, ok := bind(inner, target, source)
		if !ok {
			w.gaps = append(w.gaps, Gap{Host: host.id, Contributor: ref, Reason: GapGeneric})
			return
		}
		inner = bound
	}
	next := source
	policy := source.Members()
	if target.Target.Lang != source.Lang() {
		// A contributor in another language walks under its own
		// policy: Go promotion says nothing about a proto message.
		next = w.b.forLang(target.Target.Lang)
		policy = next.Members()
	}
	// The path is exactly as long as its capacity, so a caller
	// appending to one member's Through never writes into the
	// members that share it.
	path := make([]symbol.Identity, len(through)+1)
	copy(path, through)
	path[len(through)] = target.Target
	w.visit(inner, policy, next, path, depth+1, viaOptional || ref.Form == symbol.FormOptional, ref)
}

// namedUnder returns the named reference a structural one wraps in
// one child, or the reference itself.
func namedUnder(ref *node.TypeRef) *node.TypeRef {
	for ref != nil && ref.Form != symbol.FormNamed && len(ref.Elems) == 1 {
		ref = ref.Elems[0]
	}
	return ref
}

// bind restates a generic contributor's members and its own
// contributors with the reference's arguments substituted through
// the language's generics capability, so a chain of generic
// contributors binds each level to the arguments of the level
// above. It reports false where a parameter stays unbound or the
// language returns no capability.
func bind(inner membered, ref *node.TypeRef, source SourceRules) (membered, bool) {
	generics, held := source.(GenericsRules)
	if !held || len(ref.Args) != len(inner.params) {
		return membered{}, false
	}
	out := inner
	out.embeds = make([]*node.Embed, 0, len(inner.embeds))
	for _, e := range inner.embeds {
		if e == nil {
			continue
		}
		c := *e
		c.Ref = generics.Substitute(e.Ref, inner.params, ref.Args)
		out.embeds = append(out.embeds, &c)
	}
	out.extends = substituteRefs(generics, inner.extends, inner.params, ref.Args)
	out.implements = substituteRefs(generics, inner.implements, inner.params, ref.Args)
	out.fields = make([]*node.Field, 0, len(inner.fields))
	for _, f := range inner.fields {
		if f == nil {
			continue
		}
		c := *f
		c.Type = generics.Substitute(f.Type, inner.params, ref.Args)
		out.fields = append(out.fields, &c)
	}
	out.methods = make([]*node.Method, 0, len(inner.methods))
	for _, m := range inner.methods {
		if m == nil {
			continue
		}
		c := *m
		c.Params = substituteParams(generics, m.Params, inner.params, ref.Args)
		c.Returns = substituteReturns(generics, m.Returns, inner.params, ref.Args)
		out.methods = append(out.methods, &c)
	}
	return out, true
}

// substituteRefs restates a reference list with arguments bound.
func substituteRefs(
	g GenericsRules, refs []*node.TypeRef, params []*node.TypeParam, args []*node.TypeRef,
) []*node.TypeRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]*node.TypeRef, 0, len(refs))
	for _, r := range refs {
		if r != nil {
			out = append(out, g.Substitute(r, params, args))
		}
	}
	return out
}

// substituteParams restates a parameter list with arguments bound.
func substituteParams(g GenericsRules, ps []*node.Param, params []*node.TypeParam, args []*node.TypeRef) []*node.Param {
	if len(ps) == 0 {
		return nil
	}
	out := make([]*node.Param, 0, len(ps))
	for _, p := range ps {
		if p == nil {
			continue
		}
		c := *p
		c.Type = g.Substitute(p.Type, params, args)
		out = append(out, &c)
	}
	return out
}

// substituteReturns restates a return list with arguments bound.
func substituteReturns(
	g GenericsRules, rs []*node.Return, params []*node.TypeParam, args []*node.TypeRef,
) []*node.Return {
	if len(rs) == 0 {
		return nil
	}
	out := make([]*node.Return, 0, len(rs))
	for _, r := range rs {
		if r == nil {
			continue
		}
		c := *r
		c.Type = g.Substitute(r.Type, params, args)
		out = append(out, &c)
	}
	return out
}

// record files one arrival under its name.
func (w *walk) record(name string, m Member, via *node.TypeRef) {
	at := len(w.arrivals)
	w.arrivals = append(w.arrivals, arrival{member: m, via: via, next: -1})
	c, met := w.names[name]
	if met {
		w.arrivals[c.last].next = at
		c.last = at
	} else {
		c = chain{first: at, last: at}
		w.order = append(w.order, name)
	}
	w.names[name] = c
}

// each calls fn for every arrival of a name, in arrival order,
// stopping early when fn reports false.
func (w *walk) each(name string, fn func(a *arrival) bool) {
	for i := w.names[name].first; i >= 0; i = w.arrivals[i].next {
		if !fn(&w.arrivals[i]) {
			return
		}
	}
}

// settle applies the shadowing rule to one name's arrivals and
// appends the members that survive to dst: a declared member always
// takes its name or its signature, and on an interface one name
// with one signature arriving twice is one member.
func (w *walk) settle(dst []Member, name string, policy MemberPolicy, iface bool) []Member {
	if iface {
		w.dedupe(name)
	}
	if policy.Shadowing == ShadowOverride {
		return w.nearestPerSignature(dst, name)
	}
	count, shallowest := 0, -1
	w.each(name, func(a *arrival) bool {
		if !a.dropped {
			count++
			if shallowest < 0 || a.member.Depth < shallowest {
				shallowest = a.member.Depth
			}
		}
		return true
	})
	if count == 0 {
		return dst
	}
	if count == 1 || shallowest == 0 {
		// The one arrival, or every declared one, takes the name.
		return w.appendAt(dst, name, shallowest)
	}
	switch policy.Shadowing {
	case ShadowMerge:
		return w.appendAt(dst, name, -1)
	case ShadowLinearise:
		return w.appendFirstAt(dst, name, shallowest)
	case ShadowPromote:
		if w.countAt(name, shallowest) != 1 {
			return dst // two at one depth cancel both
		}
		return w.appendAt(dst, name, shallowest)
	default:
		return dst
	}
}

// nearestPerSignature appends one member per distinct signature
// among a name's surviving arrivals: the shallowest arrival, and the
// first of those in arrival order. Each overload keeps its nearest
// declaration. A field and an embedded field have no signature, so
// fields settle by name alone.
func (w *walk) nearestPerSignature(dst []Member, name string) []Member {
	var nearest []int
	for i := w.names[name].first; i >= 0; i = w.arrivals[i].next {
		a := &w.arrivals[i]
		if a.dropped {
			continue
		}
		held := false
		for k, j := range nearest {
			if w.arrivals[j].signature() != a.signature() {
				continue
			}
			if a.member.Depth < w.arrivals[j].member.Depth {
				nearest[k] = i
			}
			held = true
			break
		}
		if !held {
			nearest = append(nearest, i)
		}
	}
	for _, i := range nearest {
		dst = append(dst, w.arrivals[i].member)
	}
	return dst
}

// appendFirstAt appends a name's first surviving arrival at one
// depth to dst.
func (w *walk) appendFirstAt(dst []Member, name string, depth int) []Member {
	w.each(name, func(a *arrival) bool {
		if a.dropped || a.member.Depth != depth {
			return true
		}
		dst = append(dst, a.member)
		return false
	})
	return dst
}

// appendAt appends a name's surviving arrivals at one depth to dst,
// or every surviving arrival for a negative depth.
func (w *walk) appendAt(dst []Member, name string, depth int) []Member {
	w.each(name, func(a *arrival) bool {
		if !a.dropped && (depth < 0 || a.member.Depth == depth) {
			dst = append(dst, a.member)
		}
		return true
	})
	return dst
}

// countAt counts a name's surviving arrivals at one depth.
func (w *walk) countAt(name string, depth int) int {
	n := 0
	w.each(name, func(a *arrival) bool {
		if !a.dropped && a.member.Depth == depth {
			n++
		}
		return true
	})
	return n
}

// dedupe drops a name's later arrivals with the first's signature,
// and files a conflict for a later arrival whose signature differs
// while both are stated. The conflict names the contributor the
// later arrival came by and the host whose list holds it.
func (w *walk) dedupe(name string) {
	head := &w.arrivals[w.names[name].first]
	for i := head.next; i >= 0; i = w.arrivals[i].next {
		a := &w.arrivals[i]
		if a.signature() == head.signature() && a.signature() != "" {
			a.dropped = true
			continue
		}
		if a.signature() != "" && head.signature() != "" {
			w.gaps = append(w.gaps, Gap{Host: w.hostOf(a), Contributor: a.via, Reason: GapConflict})
			a.dropped = true
		}
	}
}

// hostOf returns the type whose list holds the contributor an
// arrival came by: the second-to-last type on its path, or the root
// for a member one contributor away or declared.
func (w *walk) hostOf(a *arrival) symbol.Identity {
	if n := len(a.member.Through); n >= 2 {
		return a.member.Through[n-2]
	}
	return w.root
}

// signature spells a method's parameter and return types by what
// they name, each parameter's variadic kind included, so two
// arrivals of one name compare equal only when they state one
// signature.
func signature(m *node.Method) string {
	var b strings.Builder
	for _, p := range m.Params {
		if p != nil {
			b.WriteString(strconv.Itoa(int(p.Variadic)))
			typeKey(&b, p.Type)
		}
		b.WriteByte(',')
	}
	b.WriteByte(')')
	for _, r := range m.Returns {
		if r != nil {
			typeKey(&b, r.Type)
		}
		b.WriteByte(',')
	}
	return b.String()
}

// typeKey writes what a reference names: a resolved name by its
// target, an unresolved one by its spelling, and a structural one
// by its form, its length and split, and its children. Type
// arguments follow in brackets. Two references spelled alike in two
// packages key apart by their targets.
func typeKey(b *strings.Builder, ref *node.TypeRef) {
	if ref == nil {
		return
	}
	switch {
	case !ref.Target.IsZero():
		b.WriteString(ref.Target.String())
	case ref.Form != symbol.FormNamed:
		b.WriteString(strconv.Itoa(int(ref.Form)))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(ref.Length))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(ref.Split))
	default:
		b.WriteString(ref.Spelling)
	}
	if len(ref.Elems)+len(ref.Args) == 0 {
		return
	}
	b.WriteByte('[')
	for _, e := range ref.Elems {
		typeKey(b, e)
		b.WriteByte(',')
	}
	for _, a := range ref.Args {
		typeKey(b, a)
		b.WriteByte(',')
	}
	b.WriteByte(']')
}
