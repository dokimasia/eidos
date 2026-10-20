// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells the filename of the module of a unit. Python names a
// module in snake case, so the stem of the unit's [plugin.Unit.FileKey], the
// family word and the tag join with underscores and convert as one. The
// store unit of the stub family has the filename store_stub.py. A
// per-package or a per-plan unit has no file key, so its filename is the
// word and the tag alone.
//
// The leading and the trailing underscores of the join remain, so the module
// _impl of a source keeps its name. A stem that starts and ends with two
// underscores keeps its spelling where it is the one part, as __init__
// does. A name that is a keyword of Python takes a trailing underscore, as
// in class_.py, because an import cannot name a module class.
//
// # Allocation contract
//
// Filename allocates the list of the parts of the filename, the join of
// more than one part, the conversion of a join that is not snake case
// already, and the name with its extension: three allocations for a
// per-source unit of snake-case parts.
func Filename(u plugin.Unit) string {
	return filename(u, python.Extension)
}

// StubFilename spells the filename of the stub file of a unit as [Filename]
// spells the filename of its module, with the extension .pyi. It allocates
// what Filename allocates.
func StubFilename(u plugin.Unit) string {
	return filename(u, python.StubExtension)
}

// filename spells the filename of the unit u by the rules of [Filename], with
// the extension ext.
func filename(u plugin.Unit, ext string) string {
	parts := naming.FilenameParts(u.FileKey(), u.Word, u.Tag)
	joined := strings.Join(parts, underscore)
	if len(parts) == 1 && len(joined) > 2*len(mangled) &&
		strings.HasPrefix(joined, mangled) && strings.HasSuffix(joined, mangled) {

		return joined + ext
	}
	trimmed := strings.TrimLeft(joined, underscore)
	core := strings.TrimRight(trimmed, underscore)
	lead, trail := joined[:len(joined)-len(trimmed)], trimmed[len(core):]
	stem := caser.Snake(core)
	if lead == "" && trail == "" && keywords[stem] {
		return stem + underscore + ext
	}
	return lead + stem + trail + ext
}
