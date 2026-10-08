// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// templateTree is one template tree a plan renders through: the
// generator that declares it, the target of the plan's backend, and the
// tree the generator returns for that target.
type templateTree struct {
	plugin plugin.ID
	target plugin.Target
	tree   fs.FS
}

// templateTrees returns the template trees the plans render through: for
// each generator of each plan that declares presentation, the tree it
// returns for the target of the plan's backend, once for each generator
// and target, sorted by generator, then target.
func templateTrees(plans []compiledPlan) []templateTree {
	var out []templateTree
	for _, p := range plans {
		target := p.backend.Target()
		for _, g := range p.entries {
			provider, declares := g.run.(plugin.TemplateProvider)
			if !declares {
				continue
			}
			if tree, held := provider.Templates(target); held {
				out = append(out, templateTree{plugin: g.name, target: target, tree: tree})
			}
		}
	}
	slices.SortFunc(out, func(a, b templateTree) int {
		return cmp.Or(cmp.Compare(a.plugin, b.plugin), cmp.Compare(a.target, b.target))
	})
	return slices.CompactFunc(out, func(a, b templateTree) bool {
		return a.plugin == b.plugin && a.target == b.target
	})
}

// appendTree appends the fold of every file of a template tree to dst in
// lexical order, each file's path and the SHA-256 of its bytes, and
// returns the extended buffer. A walk or a read that fails appends the
// error's text and ends the fold of the tree.
func appendTree(dst []byte, tree fs.FS) []byte {
	err := fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(tree, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		dst = appendPart(dst, path)
		dst = appendPart(dst, sum[:])
		return nil
	})
	if err != nil {
		dst = appendPart(dst, err.Error())
	}
	return dst
}

// appendPart appends one part to dst behind its length, eight bytes in
// little-endian order, so two sequences of parts cannot trade bytes, and
// returns the extended buffer. It allocates only where dst lacks the
// room.
func appendPart[B ~string | ~[]byte](dst []byte, part B) []byte {
	dst = binary.LittleEndian.AppendUint64(dst, uint64(len(part)))
	return append(dst, part...)
}

// planNames spells the names of the plans at the indexes, sorted and
// quoted, for the fold.
func planNames(plans []compiledPlan, at []int) string {
	names := make([]string, 0, len(at))
	for _, i := range at {
		names = append(names, plans[i].name)
	}
	slices.Sort(names)
	return fmt.Sprintf("%q", names)
}

// pluginEntry spells one plugin for the fold: its name, its version
// where it declares one, and its options in their canonical encoding.
func pluginEntry(name plugin.ID, run any, encoded []byte) string {
	var b strings.Builder
	b.WriteString(string(name))
	b.WriteByte(0)
	if v, versioned := run.(plugin.Versioned); versioned {
		b.WriteString(v.Version())
	}
	b.WriteByte(0)
	b.Write(encoded)
	return b.String()
}

// fingerprint folds the composition's brand, workspace name, frontends
// with their options' encodings, ignored spellings, scheduled plugins,
// plans and checks, each plugin with its options' encoding, and every key
// of the registry with the kinds it may be stamped on, its group and its
// contract.
func (b *Builder) fingerprint(
	annotate []annEntry, plans []compiledPlan, checks []compiledCheck, options map[plugin.ID][]byte,
	fronts [][]byte, keys *meta.Registry,
) []byte {
	entries := make([]string, 0, len(annotate)+len(plans)+len(checks)+4)
	entries = append(entries, "brand\x00"+string(b.brand), "workspace\x00"+b.id)
	for name := range keys.Keys() {
		id, _ := keys.Resolve(name)
		spec, _ := keys.Spec(id)
		entry := fmt.Sprintf("key\x00%s\x00%v\x00%s", spec.Name, spec.Kinds, spec.Group)
		if c := spec.Contract; c != nil {
			entry += fmt.Sprintf("\x00%v\x00%s\x00%v", c.On, c.By, c.Severity)
		}
		entries = append(entries, entry)
	}
	if len(b.frontends) > 0 {
		var sb strings.Builder
		sb.WriteString("frontends\x00")
		for i, f := range b.frontends {
			sb.WriteString(pluginEntry(f.Name(), f, fronts[i]))
			sb.WriteByte(0)
			fmt.Fprintf(&sb, "%q", f.Selection())
			sb.WriteByte(0)
		}
		entries = append(entries, sb.String())
	}
	if len(b.ignored) > 0 {
		ignored := make([]string, 0, len(b.ignored))
		for _, n := range b.ignored {
			ignored = append(ignored, string(n))
		}
		slices.Sort(ignored)
		entries = append(entries, fmt.Sprintf("ignored\x00%q", slices.Compact(ignored)))
	}
	for _, a := range annotate {
		entries = append(entries, "annotator\x00"+pluginEntry(a.name, a.run, options[a.name]))
	}
	var routing []byte
	for _, p := range plans {
		var sb strings.Builder
		sb.WriteString("plan\x00")
		sb.WriteString(p.name)
		sb.WriteByte(0)
		sb.WriteString(p.sources.fold())
		sb.WriteByte(0)
		sb.WriteString(planNames(plans, p.deps))
		sb.WriteByte(0)
		routing, _ = p.routing.AppendBinary(routing[:0])
		sb.Write(routing)
		sb.WriteByte(0)
		for _, g := range p.entries {
			sb.WriteString(pluginEntry(g.name, g.run, options[g.name]))
			sb.WriteByte(0)
		}
		sb.WriteString("backend\x00")
		sb.WriteString(pluginEntry(p.backend.Name(), p.backend, options[p.backend.Name()]))
		entries = append(entries, sb.String())
	}
	for _, c := range checks {
		entries = append(entries,
			"check\x00"+pluginEntry(c.name, c.run, options[c.name])+"\x00"+planNames(plans, c.reads))
	}
	slices.Sort(entries)

	size := 0
	for _, e := range entries {
		size += 8 + len(e)
	}
	fold := make([]byte, 0, size)
	for _, e := range entries {
		fold = appendPart(fold, e)
	}
	sum := sha256.Sum256(fold)
	return sum[:]
}

// Fingerprint returns the composition's fingerprint, taken at Build: the
// brand and the workspace's name; each frontend in load order with its
// name, version, canonical options and selection; every scheduled
// annotator, generator and backend with its version and canonical
// options; each plan's name, sources, dependencies and layout
// configuration; each workspace check with the plans it reads; the
// ignored directive spellings; and every registered key with the kinds it
// may be stamped on, its group and its contract, folded in sorted order.
//
// Each generation of the sealed state records the SHA-256 of the
// fingerprint and of each template tree the plans render through, file
// by file, as the run reads the trees. A run over a generation that
// records another digest ignores the generation, reports [ColdState] and
// runs cold, so a change of a plan's scope or layout, and an edit of a
// template on disk, also run cold. What a plugin's code declares, such
// as its schemas, is in the executable, whose digest the generation
// records too. The keys are in the fold all the same, because one
// executable can build compositions that register different keys, and a
// warm run audits only the subjects that changed under the contracts that
// the generation records. The fingerprint is taken over the options as
// the configuration left them. It is a SHA-256 digest, and every call
// returns a fresh copy, one allocation.
func (w *Workspace) Fingerprint() []byte { return slices.Clone(w.fingerprint) }

// composition returns the digest a generation's header records: the
// SHA-256 of the fingerprint, then of each template tree the plans
// render through, as the run reads the tree now, so an edit of a
// template between two runs of one workspace changes it. A tree folds
// its files in lexical order, each path beside the SHA-256 of its bytes.
// A tree or a file that does not read folds the error's text in place of
// what it did not read, so the run proceeds and the render reports the
// failure, as a run without a sealed state does. It reads every file of
// every tree once, and allocates nothing for a composition without
// trees.
func (w *Workspace) composition() [sha256.Size]byte {
	fold := slices.Clip(w.fingerprint)
	for _, t := range w.trees {
		fold = appendPart(fold, t.plugin)
		fold = appendPart(fold, t.target)
		fold = appendTree(fold, t.tree)
	}
	return sha256.Sum256(fold)
}
