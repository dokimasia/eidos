// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Target is what [Workspace.Explain] explains. A target sets exactly one of
// four forms:
//
//   - Path is the path of a generated file, relative to the workspace root
//     and separated by slashes.
//   - Identity is a declaration. [symbol.Parse] returns most identities
//     without a kind, and an identity without a kind names every
//     declaration whose other fields are equal.
//   - Key with At is the fact of a registered key on each declaration at a
//     position.
//   - Code with At is the findings of a code at a position.
//
// The file of At is relative to the workspace root. A zero column matches
// every column of the line.
type Target struct {
	Path     string
	Identity symbol.Identity
	Key      meta.KeyName
	Code     diag.Code
	At       position.Pos
}

// ParseTarget parses the argument of the explain command into a target. The
// argument has one of four forms:
//
//   - CODE@FILE:LINE or CODE@FILE:LINE:COLUMN is a code at a position, when
//     [diag.ParseCode] accepts the part before the first @.
//   - KEY@FILE:LINE or KEY@FILE:LINE:COLUMN is a key at a position, for any
//     other part before the first @.
//   - An argument that [symbol.Parse] accepts is an identity, when a
//     frontend of the composition loads its language or a rules value
//     declares it.
//   - Any other argument is a path.
//
// ParseTarget returns the path and the file of the position as the argument
// has them, and the caller converts them to paths relative to the workspace
// root. ParseTarget splits a position at its last colons, so a file name
// that contains a colon stays whole.
//
// Error modes: a position without a file or a line, a line or a column
// below one, and a key that the composition does not register.
func (w *Workspace) ParseTarget(s string) (Target, error) {
	left, at, positioned := strings.Cut(s, "@")
	if !positioned {
		if id, err := symbol.Parse(s); err == nil && languages(w.frontends, w.rules)[id.Lang] {
			return Target{Identity: id}, nil
		}
		return Target{Path: s}, nil
	}
	pos, err := parsePosition(at)
	if err != nil {
		return Target{}, err
	}
	if code, cerr := diag.ParseCode(left); cerr == nil {
		return Target{Code: code, At: pos}, nil
	}
	key := meta.KeyName(left)
	if _, registered := w.keys.Resolve(key); !registered {
		return Target{}, fmt.Errorf(
			"workspace: %q is neither a diagnostic code nor a key that the composition registers", left,
		)
	}
	return Target{Key: key, At: pos}, nil
}

// checkTarget returns an error for a target that Explain refuses. Explain
// refuses these targets:
//
//   - a target that does not set exactly one of the four forms
//   - a key or a code without a file and a line, and a path or an identity
//     with a position
//   - a key that the composition does not register
func (w *Workspace) checkTarget(t Target) error {
	forms := 0
	for _, set := range [...]bool{t.Path != "", !t.Identity.IsZero(), t.Key != "", !t.Code.IsZero()} {
		if set {
			forms++
		}
	}
	positioned := t.Key != "" || !t.Code.IsZero()
	switch {
	case forms != 1:
		return errors.New("workspace: a target sets exactly one of a path, an identity, a key and a code")
	case positioned && (t.At.File == "" || t.At.Line < 1):
		return fmt.Errorf("workspace: the target position %v has no file or no line", t.At)
	case !positioned && t.At != (position.Pos{}):
		return fmt.Errorf("workspace: a target of a path or an identity has no position, and this target has %v", t.At)
	}
	if _, registered := w.keys.Resolve(t.Key); t.Key != "" && !registered {
		return fmt.Errorf("workspace: the composition registers no key %s", t.Key)
	}
	return nil
}

// parsePosition parses FILE:LINE or FILE:LINE:COLUMN. The number after the
// last colon is the line. When the text between the last two colons is also
// a number, that number is the line and the last number is the column. A
// file name that contains a colon stays whole.
//
// Error modes: a position without a file or a line, and a line or a column
// below one.
func parsePosition(s string) (position.Pos, error) {
	refused := fmt.Errorf("workspace: %q is not a position: use FILE:LINE or FILE:LINE:COLUMN, from line 1", s)
	cut := strings.LastIndexByte(s, ':')
	if cut < 1 {
		return position.Pos{}, refused
	}
	head := s[:cut]
	last, err := strconv.Atoi(s[cut+1:])
	if err != nil || last < 1 {
		return position.Pos{}, refused
	}
	if mid := strings.LastIndexByte(head, ':'); mid > 0 {
		if line, lerr := strconv.Atoi(head[mid+1:]); lerr == nil {
			if line < 1 {
				return position.Pos{}, refused
			}
			return position.Pos{File: head[:mid], Line: line, Col: last}, nil
		}
	}
	return position.Pos{File: head, Line: last}, nil
}
