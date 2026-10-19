// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// statements are the version statements of the versions of protobuf, by
// the names of the versions.
var statements = map[string]string{
	protobuf.Proto2:      `syntax = "` + protobuf.Proto2 + `";`,
	protobuf.Proto3:      `syntax = "` + protobuf.Proto3 + `";`,
	protobuf.Edition2023: `edition = "` + protobuf.Edition2023 + `";`,
	protobuf.Edition2024: `edition = "` + protobuf.Edition2024 + `";`,
	protobuf.Edition2026: `edition = "` + protobuf.Edition2026 + `";`,
}

// Syntax returns the version statement of the file whose header is h: the
// syntax statement of proto2 and proto3, and the edition statement of an
// edition. A header without the marker protobuf:syntax states proto3.
// Syntax allocates nothing.
//
// Error modes: the refusals of [FileScope].
func Syntax(h *emit.File) (string, error) {
	s, err := FileScope(h)
	if err != nil {
		return "", err
	}
	return statements[s.Version().Name], nil
}

// UseImports records the import of each import marker of the header h into
// the import set of the file, with the modifier of the marker, and returns
// the empty string. The file skeleton calls it before the import block. A
// file that the header imports with the modifiers public and option, or
// weak and option, takes the modifier public or weak, and a file that it
// imports with option and without a modifier takes a regular import, as
// [Imports] states.
//
// Error modes: a weak import in edition 2024 or 2026, which remove it, an
// option import before edition 2024, a file that the header imports both
// public and weak, and the refusals of [FileScope].
func (v *Vocabulary) UseImports(h *emit.File) (string, error) {
	s, err := FileScope(h)
	if err != nil {
		return "", err
	}
	version := s.Version().Name
	modern := version == protobuf.Edition2024 || version == protobuf.Edition2026
	for i, a := range h.Annotations {
		if a.Name != string(protobuf.ImportMarker) || len(a.Args) != 1 {
			continue
		}
		modifier, path := importOf(a.Args[0])
		switch {
		case modifier == weakModifier && modern:
			return "", fmt.Errorf(refusalPrefix+"edition %s removes the weak import, and %s imports %s weak",
				version, headerName, path)
		case modifier == optionModifier && !modern:
			return "", fmt.Errorf(refusalPrefix+"%s has no option import, and %s imports %s for its options",
				version, headerName, path)
		}
		exclusive := modifier == publicModifier || modifier == weakModifier
		for _, prior := range h.Annotations[:i] {
			if prior.Name != string(protobuf.ImportMarker) || len(prior.Args) != 1 {
				continue
			}
			was, at := importOf(prior.Args[0])
			if exclusive && at == path && was != modifier && (was == publicModifier || was == weakModifier) {
				return "", fmt.Errorf(refusalPrefix+"%s imports %s as %s and as %s, and an import has one modifier",
					headerName, path, was, modifier)
			}
		}
		if modifier == "" {
			v.set.Add(path)
		} else {
			v.set.AddNamed(path, modifier)
		}
	}
	return "", nil
}

// FileOptions returns one option statement for each option of the header
// h, in the order of buf format: the plain options in the order of their
// names, and then the custom options in the order of their names. Options
// of one name keep the order of the header. A marker states no option.
// FileOptions allocates the sorted options and the statements, and nothing
// for a header without an option.
func FileOptions(h *emit.File) string {
	n := 0
	for _, a := range h.Annotations {
		if !strings.HasPrefix(a.Name, protobuf.MarkerPrefix) {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	options := make(symbol.Annotations, 0, n)
	for _, a := range h.Annotations {
		if !strings.HasPrefix(a.Name, protobuf.MarkerPrefix) {
			options = append(options, a)
		}
	}
	slices.SortStableFunc(options, func(a, b symbol.Annotation) int {
		custom, other := strings.HasPrefix(
			a.Name,
			string(extensionOpen),
		), strings.HasPrefix(
			b.Name,
			string(extensionOpen),
		)
		switch {
		case custom == other:
			return strings.Compare(a.Name, b.Name)
		case custom:
			return 1
		default:
			return -1
		}
	})
	return optionStatements(options, nil, "")
}

// importOf returns the modifier and the path of the argument arg of an
// import marker. An argument without a space is a path without a modifier.
func importOf(arg string) (modifier, path string) {
	modifier, path, modified := strings.Cut(arg, importSep)
	if !modified {
		return "", arg
	}
	return modifier, path
}
