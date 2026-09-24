// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
)

// Fingerprint returns the composition's fingerprint: every
// scheduled annotator and generator with its version and canonical
// options, each plan's name and backend, folded in sorted order. A
// load takes it as its plugin-set, so two compositions never share
// unit keys and a composition change re-keys every unit. What a
// plugin declares beyond its identity — keys, schemas, templates —
// is covered by its version under the bump-on-any-change rule, so
// the fingerprint needs no registry walk. A plan's scope is a
// function and spells nothing, so a scope change re-keys only
// through the plan's own name. The fingerprint is taken at Build,
// over the options as the config left them, and every call returns
// a fresh copy.
func (w *Workspace) Fingerprint() []byte { return slices.Clone(w.fingerprint) }

// fingerprintOf folds the composition's scheduled plugins and plans
// with each plugin's options encoding.
func fingerprintOf(
	annotate []annEntry, plans []compiledPlan, options map[plugin.ID][]byte,
) []byte {
	entries := make([]string, 0, len(annotate)+len(plans))
	for _, a := range annotate {
		entries = append(entries, "annotator\x00"+pluginEntry(a.name, a.run, options))
	}
	for _, p := range plans {
		var b strings.Builder
		b.WriteString("plan\x00")
		b.WriteString(p.name)
		b.WriteByte(0)
		for _, g := range p.entries {
			b.WriteString(pluginEntry(g.name, g.run, options))
			b.WriteByte(0)
		}
		b.WriteString("backend\x00")
		b.WriteString(pluginEntry(p.backend.Name(), p.backend, options))
		entries = append(entries, b.String())
	}
	slices.Sort(entries)

	h := sha256.New()
	for _, e := range entries {
		var n [8]byte
		binary.LittleEndian.PutUint64(n[:], uint64(len(e)))
		h.Write(n[:])
		h.Write([]byte(e))
	}
	return h.Sum(nil)
}

// pluginEntry spells one scheduled plugin for the fold: its name,
// its version where it declares one, and its options in the
// canonical encoding the configure step took.
func pluginEntry(name plugin.ID, run any, options map[plugin.ID][]byte) string {
	var b strings.Builder
	b.WriteString(string(name))
	b.WriteByte(0)
	if v, versioned := run.(plugin.Versioned); versioned {
		b.WriteString(v.Version())
	}
	b.WriteByte(0)
	b.Write(options[name])
	return b.String()
}
