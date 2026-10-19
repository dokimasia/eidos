// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The syntax of an option: the statement of an option of a block, and the
// compact options of a field and of an enum value in brackets.
const (
	optionKeyword = "option "
	optionAssign  = " = "
	compactOpen   = " ["
	compactClose  = "]"
	// argsJoin joins the arguments of an annotation that states more than
	// one value, which the check of the written file then refuses.
	argsJoin = ", "
	// defaultOption is the option of the default value of a field, which a
	// field states through its value.
	defaultOption = "default"
)

// feature is a feature that the backend adds to a declaration, because the
// value that the features of its scope resolve to differs from the value
// that its shape or its marker needs.
type feature struct {
	name  string
	value string
}

// optionStatements writes one option statement behind indent for each
// option of as, and then for each feature of added. A marker writes no
// option. The value of an option is protobuf syntax, so optionStatements
// writes it as it is. It sizes the statements once, allocates them, and
// allocates nothing where there is none.
func optionStatements(as symbol.Annotations, added []feature, indent string) string {
	statement := len(indent) + len(optionKeyword) + len(optionAssign) + len(statementEnd)
	size := 0
	for _, a := range as {
		if !strings.HasPrefix(a.Name, protobuf.MarkerPrefix) {
			size += statement + len(a.Name) + joinedLen(a.Args)
		}
	}
	for _, f := range added {
		size += statement + len(f.name) + len(f.value)
	}
	if size == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(size)
	for _, a := range as {
		if strings.HasPrefix(a.Name, protobuf.MarkerPrefix) {
			continue
		}
		b.WriteString(indent)
		writeOption(&b, optionKeyword, a.Name, strings.Join(a.Args, argsJoin), statementEnd)
	}
	for _, f := range added {
		b.WriteString(indent)
		writeOption(&b, optionKeyword, f.name, f.value, statementEnd)
	}
	return b.String()
}

// compactOptions writes the compact options of a field, an enum value or a
// group in brackets after a space: the default value where value is not
// empty, each option of as, and each feature of added. It writes one option
// on the line of its declaration, and two or more options on lines of their
// own behind the indentation of a member, with a comma after each option
// but the last, as buf format writes them. indent is the indentation of the
// line of the declaration. compactOptions writes nothing where there is no
// option. It sizes the brackets once and allocates them alone.
func compactOptions(value string, as symbol.Annotations, added []feature, indent string) string {
	n := len(added)
	size := 0
	if value != "" {
		n++
		size += len(defaultOption) + len(optionAssign) + len(value)
	}
	for _, a := range as {
		if !strings.HasPrefix(a.Name, protobuf.MarkerPrefix) {
			n++
			size += len(a.Name) + len(optionAssign) + joinedLen(a.Args)
		}
	}
	for _, f := range added {
		size += len(f.name) + len(optionAssign) + len(f.value)
	}
	if n == 0 {
		return ""
	}
	size += len(compactOpen) + len(compactClose)
	if n > 1 {
		// Each option takes a comma and a line of its own, and the closing
		// bracket takes a line of its own.
		size += n*(2+len(indent)+len(memberIndent)) + 1 + len(indent)
	}
	var b strings.Builder
	b.Grow(size)
	b.WriteString(compactOpen)
	written := 0
	entry := func(name, v string) {
		if n > 1 {
			if written > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
			b.WriteString(indent)
			b.WriteString(memberIndent)
		}
		writeOption(&b, "", name, v, "")
		written++
	}
	if value != "" {
		entry(defaultOption, value)
	}
	for _, a := range as {
		if !strings.HasPrefix(a.Name, protobuf.MarkerPrefix) {
			entry(a.Name, strings.Join(a.Args, argsJoin))
		}
	}
	for _, f := range added {
		entry(f.name, f.value)
	}
	if n > 1 {
		b.WriteByte('\n')
		b.WriteString(indent)
	}
	b.WriteString(compactClose)
	return b.String()
}

// joinedLen returns the length of the value of an option whose annotation
// states the arguments args: the arguments joined by argsJoin.
func joinedLen(args []string) int {
	n := 0
	for i, a := range args {
		if i > 0 {
			n += len(argsJoin)
		}
		n += len(a)
	}
	return n
}

// writeOption writes one option into b: before, the name, the assignment,
// the value and after.
func writeOption(b *strings.Builder, before, name, value, after string) {
	b.WriteString(before)
	b.WriteString(name)
	b.WriteString(optionAssign)
	b.WriteString(value)
	b.WriteString(after)
}

// statedFeatures returns the features that the annotations as state
// themselves, applied to the zero features, so a feature that as does not
// state is empty.
func statedFeatures(as symbol.Annotations) protobuf.Features {
	var f protobuf.Features
	for _, a := range as {
		if (a.Name == protobuf.FeaturesOption || strings.HasPrefix(a.Name, featuresPrefix)) && len(a.Args) == 1 {
			f = f.Apply(a.Name, a.Args[0])
		}
	}
	return f
}

// marked reports whether the annotations as state the marker m.
func marked(as symbol.Annotations, m protobuf.Marker) bool {
	for _, a := range as {
		if a.Name == string(m) {
			return true
		}
	}
	return false
}
