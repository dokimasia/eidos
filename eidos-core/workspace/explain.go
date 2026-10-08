// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// explainCaller is the caller name that Explain writes into the record of
// the lock holder.
const explainCaller = "workspace.Explain"

// ErrNoGeneration reports a workspace whose ledger has no generation to
// explain. Either no run has written a generation, or the composition has
// no ledger.
var ErrNoGeneration = errors.New("workspace: the ledger contains no generation to explain")

// RecordKind is the kind of execution that an [ExplainedRecord] describes.
// The zero value is not a valid kind.
type RecordKind uint8

const (
	// RecordValidation is the validation of the directives of one subject.
	RecordValidation RecordKind = 1
	// RecordInvocation is one invocation of the phase call of an annotator
	// or a generator.
	RecordInvocation RecordKind = 2
	// RecordCheck is one call of a workspace check.
	RecordCheck RecordKind = 3
	// RecordGroup is one group of files of a plan, which the run executes
	// together.
	RecordGroup RecordKind = 4
	// RecordAudit is an audit finding on a declaration that lacks a fact
	// that a contract requires.
	RecordAudit RecordKind = 5
	// RecordRegion is the parse and the link of one unit of the load.
	RecordRegion RecordKind = 6
)

// String returns the name of the kind for a report, such as "validation".
// For an undeclared kind, it returns the number in the form RecordKind(n).
func (k RecordKind) String() string {
	switch k {
	case RecordValidation:
		return "validation"
	case RecordInvocation:
		return "invocation"
	case RecordCheck:
		return "check"
	case RecordGroup:
		return "group"
	case RecordAudit:
		return "audit"
	case RecordRegion:
		return "region"
	default:
		return "RecordKind(" + strconv.Itoa(int(k)) + ")"
	}
}

// ReadGrain is the grain of a read that Explain identifies, from one
// declaration to a whole enumeration. The zero value is not a valid grain.
type ReadGrain uint8

const (
	// ReadDeclaration is a read of one declaration.
	ReadDeclaration ReadGrain = 1
	// ReadPackage is a read of one whole package.
	ReadPackage ReadGrain = 2
	// ReadFact is a read of one fact. The fact was present or absent.
	ReadFact ReadGrain = 3
	// ReadKind is an enumeration of the declarations of one kind.
	ReadKind ReadGrain = 4
	// ReadDirective is an enumeration of the declarations that have one
	// directive.
	ReadDirective ReadGrain = 5
	// ReadExport is a read of the export of one plan.
	ReadExport ReadGrain = 6
)

// String returns the name of the grain for a report, such as
// "declaration". For an undeclared grain, it returns the number in the form
// ReadGrain(n).
func (g ReadGrain) String() string {
	switch g {
	case ReadDeclaration:
		return "declaration"
	case ReadPackage:
		return "package"
	case ReadFact:
		return "fact"
	case ReadKind:
		return "kind"
	case ReadDirective:
		return "directive"
	case ReadExport:
		return "export"
	default:
		return "ReadGrain(" + strconv.Itoa(int(g)) + ")"
	}
}

// Read is one read of a record, which Explain identified from the hash that
// the generation stores.
type Read struct {
	Grain ReadGrain
	// Subject is the declaration of a declaration read or a fact read, and
	// the package of a package read. Key is the key of a fact read.
	Subject symbol.Identity
	Key     meta.KeyName
	// Kind is the kind of a kind enumeration, and Directive is the directive
	// of a directive enumeration. Plan is the plan of an export read.
	Kind      symbol.Kind
	Directive directive.Name
	Plan      string
}

// Explanation is what a generation records about a target.
type Explanation struct {
	// Generation is the name of the generation. Recorded is the anchor of
	// the run that wrote it, which is the time when the sweep of the run
	// started, minus two seconds.
	Generation string
	Recorded   time.Time
	// Files lists the generated files of the target. For a path, it lists the
	// file at the path. For a declaration, it lists the files that the plans
	// generated from the declaration.
	Files []ExplainedFile
	// Claims lists the claims on the facts of the target. The claims of each
	// fact are in rank order, and the winner comes first.
	Claims []ExplainedClaim
	// Records lists the records of the executions that produced the target,
	// read it, or reported its findings.
	Records []ExplainedRecord
}

// ExplainedFile is one generated file.
type ExplainedFile struct {
	// Entry is the manifest entry of the file, with its plan, its digest, its
	// plugins and its source declarations.
	Entry manifest.Entry
	// Contributors lists the invocations that placed a declaration into the
	// group of the file, in canonical match order. Findings lists the
	// findings that the render reported for the group.
	Contributors []plugin.MatchKey
	Findings     []diag.Diag
}

// ExplainedClaim is one claim on a fact.
type ExplainedClaim struct {
	// Key is the key of the fact. Claim is the envelope of the claim, and its
	// Subject is the declaration of the fact.
	Key   meta.KeyName
	Claim meta.Claim
	// Value is the claimed value, and nil for a drop.
	Value any
	// Winner reports whether the claim ranks first on the fact.
	Winner bool
}

// ExplainedRecord is the record of one execution in a generation.
type ExplainedRecord struct {
	Kind RecordKind
	// Plan is the plan of an invocation or a group of a plan. It is empty for
	// the invocation of an annotator and for every other kind.
	Plan string
	// Match is the match of an invocation, Check is the name of a check, and
	// Group is the key unit of a group. Subject is the subject of a
	// validation, an invocation or an audit finding.
	Match   plugin.MatchKey
	Check   plugin.ID
	Group   plugin.UnitRef
	Subject symbol.Identity
	// Findings lists the findings that the execution reported. For a target
	// of a code at a position, it lists only the findings of that code at that
	// position.
	Findings []diag.Diag
	// Reads contains the reads that Explain identified, sorted by grain and
	// then by their other fields. Unidentified is the number of reads that
	// Explain could not identify.
	Reads        []Read
	Unidentified int
}

// Explain returns what the live generation of the ledger records about t.
// It reads the generation under the lock of the ledger, as a run does, and
// records "workspace.Explain" as the caller of the lock. Explain runs no
// phase, so the explanation describes the last run that wrote a generation.
//
// The explanation depends on the form of t:
//
//   - For a path, it contains the manifest entry of the file, the
//     contributors and the findings of the group of the file, and the
//     records of the group and of each contributor.
//   - For an identity, it contains the claims on the facts of each
//     declaration that matches the identity, the records that read the
//     declaration, and the files that the plans generated from it.
//   - For a key at a position, it contains the claims on the fact of each
//     declaration at the position, and the records that read the fact.
//   - For a code at a position, it contains each record, audit finding and
//     load region that reported a finding of the code at the position.
//
// The explanation of a target that the generation does not record has no
// files, claims or records.
//
// A generation stores each read as the hash of an edge. Explain computes the
// hashes of these edges and compares them with the reads of each record:
//
//   - the declaration, package and fact edges of the subject of the record
//     and of each target declaration
//   - the edge of each kind
//   - the edge of each directive name in the recorded load
//   - the export edge of each plan
//
// Explain counts the reads that match no edge in
// [ExplainedRecord.Unidentified].
//
// Error modes:
//
//   - A target without exactly one form, or with a position that its form
//     does not take, returns an error.
//   - A key that the composition does not register returns an error.
//   - A composition without an output or a ledger, and a ledger without a
//     generation, return an error that wraps [ErrNoGeneration].
//   - A lock that another holder has returns a *[ledger.LockedError].
//   - An error of the ledger, and a damaged generation, return an error.
func (w *Workspace) Explain(ctx context.Context, t Target) (*Explanation, error) {
	if err := w.checkTarget(t); err != nil {
		return nil, err
	}
	l, release, err := w.lock(ctx, explainCaller)
	if err != nil {
		return nil, err
	}
	out, err := w.explain(ctx, l, t)
	return out, errors.Join(err, release())
}

// explain reads the live generation of a ledger and explains a target that
// [Workspace.checkTarget] accepted. The ledger is nil for a composition
// without a ledger.
func (w *Workspace) explain(ctx context.Context, l ledger.Ledger, t Target) (*Explanation, error) {
	if l == nil {
		return nil, ErrNoGeneration
	}
	g, err := state.Open(ctx, l)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %w", ErrNoGeneration, err)
	}
	if err != nil {
		return nil, fmt.Errorf("workspace: explain: %w", err)
	}
	loaded := g.Load(ctx)
	var units []load.UnitRecord
	for u, uerr := range loaded.Units() {
		if uerr != nil {
			return nil, fmt.Errorf("workspace: explain: %w", uerr)
		}
		units = append(units, u)
	}
	phases := g.Phases(ctx)
	x := &explainer{
		w:        w,
		phases:   phases,
		loaded:   loaded,
		facts:    meta.Restore(w.keys, phases),
		units:    units,
		common:   commonEdges(units, w.plans),
		subjects: map[symbol.Identity]map[state.EdgeHash]Read{},
		files:    map[string]bool{},
		out:      &Explanation{Generation: g.Name, Recorded: g.Header.Anchor},
	}
	switch {
	case t.Path != "":
		err = x.path(t.Path)
	case !t.Identity.IsZero():
		err = x.identity(t.Identity)
	case t.Key != "":
		err = x.fact(t.Key, t.At)
	default:
		err = x.finding(t.Code, t.At)
	}
	if err = cmp.Or(err, x.facts.Damaged()); err != nil {
		return nil, fmt.Errorf("workspace: explain: %w", err)
	}
	return x.out, nil
}

// explainer reads one generation for one call of [Workspace.Explain]. It
// is not safe for concurrent use.
type explainer struct {
	w      *Workspace
	phases *state.PhaseState
	loaded *state.LoadState
	// facts is the fact store that Explain restores from the generation. It
	// ranks the claims of each fact.
	facts *meta.Facts
	// units lists the unit records of the load, in splice order.
	units []load.UnitRecord
	// common maps the hash of each edge that is the same for every record to
	// its read, as [commonEdges] builds it. subjects maps the hashes of the
	// declaration, package and fact edges of each subject that Explain has
	// met.
	common   map[state.EdgeHash]Read
	subjects map[symbol.Identity]map[state.EdgeHash]Read
	// files lists the paths of the files that the explanation already
	// contains.
	files map[string]bool
	out   *Explanation
}

// recorded is the record of one execution before Explain identifies its
// reads. It contains the record, the hashes of the edges that the execution
// read, and the units that an invocation placed declarations into.
type recorded struct {
	ExplainedRecord
	reads []state.EdgeHash
	units []plugin.UnitRef
}

// path explains the generated file at a path. It adds the manifest entry of
// the file, the contributors and findings of its group, and the records of
// the group and of each contributor. When the generation does not record
// the group of the file, path adds only the entry.
func (x *explainer) path(at string) error {
	a, found, err := x.phases.Artifact(at)
	if err != nil || !found {
		return err
	}
	g, grouped, err := x.phases.Group(a.Entry.Plan, a.Group)
	if err != nil {
		return err
	}
	x.out.Files = append(x.out.Files, ExplainedFile{Entry: a.Entry, Contributors: g.Contributors, Findings: g.Findings})
	if !grouped {
		return nil
	}
	refs := make([]state.RecordRef, 0, len(g.Contributors)+1)
	refs = append(refs, state.GroupRef(g.Plan, g.Key))
	for _, m := range g.Contributors {
		refs = append(refs, state.InvocationRef(g.Plan, m))
	}
	var records []recorded
	for _, ref := range refs {
		kept, rerr := x.records(ref)
		if rerr != nil {
			return rerr
		}
		records = append(records, kept...)
	}
	x.explained(records)
	return nil
}

// identity explains each declaration that an identity names. It adds the
// claims on the facts of the declaration, the records that read it, and the
// files that the plans generated from it.
func (x *explainer) identity(id symbol.Identity) error {
	subjects, err := x.resolve(id)
	if err != nil {
		return err
	}
	for _, s := range subjects {
		for name := range x.w.keys.Keys() {
			x.claims(s, name)
		}
		var records []recorded
		if records, err = x.readersOf(state.DeclarationEdge(s)); err != nil {
			return err
		}
		x.explained(records, s)
		if err := x.generated(records); err != nil {
			return err
		}
	}
	return nil
}

// fact explains the fact of a registered key on each declaration at a
// position. It adds the claims on the fact and the records that read it.
func (x *explainer) fact(key meta.KeyName, at position.Pos) error {
	var subjects []symbol.Identity
	err := x.declarations(func(u load.UnitRecord) bool {
		return slices.ContainsFunc(u.Files, func(f plugin.SourceRef) bool { return f.Path == at.File })
	}, func(d node.Declaration) {
		p := d.Position()
		if p.File == at.File && p.Line == at.Line && (at.Col == 0 || p.Col == at.Col) {
			subjects = append(subjects, d.Identity())
		}
	})
	if err != nil {
		return err
	}
	for _, s := range subjects {
		x.claims(s, key)
		var records []recorded
		if records, err = x.readersOf(state.FactEdge(s, key)); err != nil {
			return err
		}
		x.explained(records, s)
	}
	return nil
}

// finding explains the findings of a code at a position. It adds each
// record, audit finding and load region that reported such a finding, and
// keeps only the findings of that code at that position.
func (x *explainer) finding(code diag.Code, at position.Pos) error {
	records, err := x.readersOf(state.FindingsEdge)
	if err != nil {
		return err
	}
	reported := records[:0]
	for _, r := range records {
		if r.Findings = matching(r.Findings, code, at); len(r.Findings) > 0 {
			reported = append(reported, r)
		}
	}
	x.explained(reported)
	audits, err := x.phases.Audits()
	if err != nil {
		return err
	}
	for _, a := range audits {
		if found := matching([]diag.Diag{a.Finding}, code, at); len(found) > 0 {
			audit := ExplainedRecord{Kind: RecordAudit, Subject: a.Subject, Findings: found}
			x.out.Records = append(x.out.Records, audit)
		}
	}
	for _, u := range x.units {
		if found := matching(u.Findings, code, at); len(found) > 0 {
			x.out.Records = append(x.out.Records, ExplainedRecord{Kind: RecordRegion, Findings: found})
		}
	}
	return nil
}

// resolve returns the declarations that an identity names. An identity with
// a kind names one declaration, the identity itself. An identity without a
// kind names each declaration of the recorded load whose other fields are
// equal, and resolve finds these declarations in the regions of the package
// of the identity.
func (x *explainer) resolve(id symbol.Identity) ([]symbol.Identity, error) {
	if id.Kind != symbol.KindInvalid {
		return []symbol.Identity{id}, nil
	}
	pkg := id.PackageIdentity()
	var out []symbol.Identity
	err := x.declarations(func(u load.UnitRecord) bool {
		return slices.Contains(u.Summary.Packages, pkg)
	}, func(d node.Declaration) {
		candidate := d.Identity()
		candidate.Kind = symbol.KindInvalid
		if candidate == id {
			out = append(out, d.Identity())
		}
	})
	return out, err
}

// claims adds the claims on the fact of a registered key on a subject, in
// rank order with the winner first.
func (x *explainer) claims(s symbol.Identity, name meta.KeyName) {
	k, _ := x.w.keys.Resolve(name)
	for v := range x.facts.Claims(s, k) {
		x.out.Claims = append(x.out.Claims, ExplainedClaim{Key: name, Claim: v.Claim, Value: v.Value, Winner: v.Won})
	}
}

// generated adds the files that the plans generated from a declaration. For
// each unit that an invocation in records placed a declaration into, it
// adds the files of the group that contains the unit, each file once.
func (x *explainer) generated(records []recorded) error {
	for _, r := range records {
		for _, u := range r.units {
			refs, err := x.phases.Readers(state.UnitEdge(r.Plan, u))
			if err != nil {
				return err
			}
			for _, ref := range refs {
				groups, gerr := x.phases.Groups(ref)
				if gerr != nil {
					return gerr
				}
				for _, g := range groups {
					if err := x.groupFiles(g); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// groupFiles adds each file of a group that the explanation does not
// contain yet, with the contributors and findings of the group. It skips a
// file that the generation has no artifact for.
func (x *explainer) groupFiles(g state.Group) error {
	for _, f := range g.Files {
		if x.files[f] {
			continue
		}
		x.files[f] = true
		a, found, err := x.phases.Artifact(f)
		if err != nil {
			return err
		}
		if found {
			x.out.Files = append(x.out.Files, ExplainedFile{
				Entry: a.Entry, Contributors: g.Contributors, Findings: g.Findings,
			})
		}
	}
	return nil
}

// readersOf returns the records whose reads include an edge, in the order of
// the readers row of the edge. Records whose IDs collide share a row, so
// readersOf keeps a record only when its own reads include the edge.
func (x *explainer) readersOf(edge state.EdgeHash) ([]recorded, error) {
	refs, err := x.phases.Readers(edge)
	if err != nil {
		return nil, err
	}
	var out []recorded
	for _, ref := range refs {
		found, rerr := x.records(ref)
		if rerr != nil {
			return nil, rerr
		}
		for _, r := range found {
			if slices.Contains(r.reads, edge) {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// records returns every record that the generation stores under the ID of a
// reference, in row order.
func (x *explainer) records(ref state.RecordRef) ([]recorded, error) {
	var out []recorded
	switch ref.Kind {
	case state.RecordValidation:
		vs, err := x.phases.Validations(ref)
		for _, v := range vs {
			out = append(out, recorded{
				Kind: RecordValidation, Subject: v.Subject, Findings: v.Findings, reads: v.Reads,
			})
		}
		return out, err
	case state.RecordInvocation:
		invs, err := x.phases.Invocations(ref)
		for _, inv := range invs {
			out = append(out, recorded{
				Kind: RecordInvocation, Plan: inv.Plan, Match: inv.Match, Subject: inv.Match.Subject,
				Findings: inv.Findings, reads: inv.Reads, units: inv.Units,
			})
		}
		return out, err
	case state.RecordCheck:
		checks, err := x.phases.Checks(ref)
		for _, c := range checks {
			out = append(out, recorded{Kind: RecordCheck, Check: c.Name, Findings: c.Findings, reads: c.Reads})
		}
		return out, err
	default:
		groups, err := x.phases.Groups(ref)
		for _, g := range groups {
			out = append(out, recorded{
				Kind: RecordGroup, Plan: g.Plan, Group: g.Key, Findings: g.Findings, reads: g.Reads,
			})
		}
		return out, err
	}
}

// explained identifies the reads of each record and adds the records to the
// explanation. A read is identified from the common edges, from the edges of
// the subject of the record, and from the edges of the target declarations.
// The findings edge only marks a record that reported a finding, so it does
// not count as a read.
func (x *explainer) explained(records []recorded, targets ...symbol.Identity) {
	for _, r := range records {
		out := r.ExplainedRecord
		subjects := append([]symbol.Identity{r.Subject}, targets...)
		for _, h := range r.reads {
			if h == state.FindingsEdge {
				continue
			}
			if read, identified := x.identify(h, subjects); identified {
				out.Reads = append(out.Reads, read)
				continue
			}
			out.Unidentified++
		}
		slices.SortFunc(out.Reads, func(a, b Read) int {
			return cmp.Or(
				cmp.Compare(a.Grain, b.Grain), a.Subject.Compare(b.Subject), cmp.Compare(a.Key, b.Key),
				cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Directive, b.Directive), cmp.Compare(a.Plan, b.Plan),
			)
		})
		x.out.Records = append(x.out.Records, out)
	}
}

// identify returns the read whose edge has the hash h. It searches the common
// edges and the edges of the subjects, and reports false when none of these
// edges has the hash.
func (x *explainer) identify(h state.EdgeHash, subjects []symbol.Identity) (Read, bool) {
	if read, common := x.common[h]; common {
		return read, true
	}
	for _, s := range subjects {
		edges, met := x.subjects[s]
		if !met {
			pkg := s.PackageIdentity()
			edges = map[state.EdgeHash]Read{state.DeclarationEdge(s): {Grain: ReadDeclaration, Subject: s}}
			edges[state.PackageEdge(pkg)] = Read{Grain: ReadPackage, Subject: pkg}
			for name := range x.w.keys.Keys() {
				edges[state.FactEdge(s, name)] = Read{Grain: ReadFact, Subject: s, Key: name}
			}
			x.subjects[s] = edges
		}
		if read, matched := edges[h]; matched {
			return read, true
		}
	}
	return Read{}, false
}

// declarations calls visit for each declaration with an identity in the
// region of each recorded unit that admit accepts. It decodes each such
// region.
func (x *explainer) declarations(admit func(load.UnitRecord) bool, visit func(node.Declaration)) error {
	for _, u := range x.units {
		if !admit(u) {
			continue
		}
		r, err := x.loaded.Region(u)
		if err != nil {
			return err
		}
		for _, p := range r.Packages {
			for s := range node.All(p) {
				if d, named := s.(node.Declaration); named && !d.Identity().IsZero() {
					visit(d)
				}
			}
		}
	}
	return nil
}

// commonEdges maps the hash of each edge that is the same for every record
// to its read. These edges are the edge of every kind, which commonEdges
// enumerates through [symbol.ParseKind], the edge of every directive name
// that a unit summary lists, and the export edge of every plan.
func commonEdges(units []load.UnitRecord, plans []compiledPlan) map[state.EdgeHash]Read {
	common := map[state.EdgeHash]Read{}
	for k := symbol.KindInvalid + 1; ; k++ {
		if _, known := symbol.ParseKind(k.String()); !known {
			break
		}
		common[state.KindEdge(k)] = Read{Grain: ReadKind, Kind: k}
	}
	for _, u := range units {
		for _, n := range u.Summary.Directives {
			common[state.DirectiveEdge(n)] = Read{Grain: ReadDirective, Directive: n}
		}
	}
	for i := range plans {
		common[state.ExportEdge(plans[i].name)] = Read{Grain: ReadExport, Plan: plans[i].name}
	}
	return common
}

// matching returns the findings of a code at a position. A finding matches
// when it has the same file and line, and the same column when the position
// has a column.
func matching(findings []diag.Diag, code diag.Code, at position.Pos) []diag.Diag {
	var out []diag.Diag
	for _, d := range findings {
		if d.Code == code && d.Pos.File == at.File && d.Pos.Line == at.Line && (at.Col == 0 || d.Pos.Col == at.Col) {
			out = append(out, d)
		}
	}
	return out
}
