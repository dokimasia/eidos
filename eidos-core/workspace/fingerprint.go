// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
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
// through the plan's own name.
func (w *Workspace) Fingerprint() []byte {
	entries := make([]string, 0, len(w.annotate)+len(w.plans))
	for _, a := range w.annotate {
		entries = append(entries, "annotator\x00"+pluginEntry(a.name, a.run))
	}
	for _, p := range w.plans {
		var b strings.Builder
		b.WriteString("plan\x00")
		b.WriteString(p.name)
		b.WriteByte(0)
		for _, g := range p.entries {
			b.WriteString(pluginEntry(g.name, g.run))
			b.WriteByte(0)
		}
		b.WriteString("backend\x00")
		b.WriteString(pluginEntry(p.backend.Name(), p.backend))
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
// its version where it declares one, and its populated options in
// their canonical encoding.
func pluginEntry(name plugin.ID, run any) string {
	var b strings.Builder
	b.WriteString(string(name))
	b.WriteByte(0)
	if v, versioned := run.(plugin.Versioned); versioned {
		b.WriteString(v.Version())
	}
	b.WriteByte(0)
	if o, configured := run.(plugin.OptionsProvider); configured {
		if encoded, err := json.Marshal(o.Options()); err == nil {
			b.Write(encoded)
		}
	}
	return b.String()
}
