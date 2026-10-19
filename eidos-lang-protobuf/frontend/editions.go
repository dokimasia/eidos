// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"github.com/bufbuild/protocompile/experimental/token/keyword"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// versionNamed returns the version that a statement of a keyword and a
// value states, from the table of [protobuf.EditionNamed]. It reports false
// for a statement outside the table, such as edition 2025 or the syntax
// keyword with an edition's value.
func versionNamed(stated keyword.Keyword, value string) (protobuf.Edition, bool) {
	e, known := protobuf.EditionNamed(value)
	if !known || e.Syntax != (stated == keyword.Syntax) {
		return protobuf.Edition{}, false
	}
	return e, true
}

// with returns f after the features options among opts, in order, as
// [protobuf.Features.Apply] applies each of them.
func with(f protobuf.Features, opts []option) protobuf.Features {
	for _, o := range opts {
		f = f.Apply(o.name, o.value)
	}
	return f
}

// local reports whether a message or an enum is local to its file: the
// keyword local, or the default visibility of its features where it states
// neither local nor export.
func local(f protobuf.Features, stated keyword.Keyword, nested bool) bool {
	switch stated {
	case keyword.Local:
		return true
	case keyword.Export:
		return false
	default:
		return f.LocalByDefault(nested)
	}
}
