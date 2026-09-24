// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package position

import (
	"cmp"
	"strconv"
)

// Pos locates a declaration in the workspace.
//
// File is workspace-relative and slash-separated on every platform.
// Line and Col are 1-based. The zero Pos carries no position and
// reports true from [Pos.IsZero]; comparing against it is the only
// absence check.
//
// Pos is a plain value: copy it freely, compare it with ==.
type Pos struct {
	File string `json:"file,omitzero"`
	Line int    `json:"line,omitzero"`
	Col  int    `json:"col,omitzero"`
}

// IsZero reports whether p carries no source position.
func (p Pos) IsZero() bool { return p == Pos{} }

// String renders the position as "file:line:col".
func (p Pos) String() string {
	return p.File + ":" + strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
}

// Compare orders two positions by file, then line, then column,
// returning a negative number, zero or a positive one as p sorts
// before, with or after o. The file compares bytewise, so the order
// is the same on every platform.
func (p Pos) Compare(o Pos) int {
	return cmp.Or(
		cmp.Compare(p.File, o.File),
		cmp.Compare(p.Line, o.Line),
		cmp.Compare(p.Col, o.Col),
	)
}
