// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"cmp"
	"math/bits"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the module enum that the values of an enum read: the
// function auto, the classes that decide how auto chooses a value, and the
// hook through which a class chooses the values of auto itself.
const (
	autoFunction  = "auto"
	flagClass     = "Flag"
	intFlagClass  = "IntFlag"
	strEnumClass  = "StrEnum"
	nextValueHook = "_generate_next_value_"
	callParens    = "()"
)

// decimalBase is the base of the decimal digits of an integer value.
const decimalBase = 10

// enumKind is the class of an enum that decides the values that auto gives
// its members, and the values that are out of its range.
type enumKind uint8

const (
	// plainEnum gives the highest integer value so far plus one, and 1 to
	// the first member. A value outside its members is out of range.
	plainEnum enumKind = iota
	// flagEnum gives the next power of two above the highest value, and 1
	// to the first member. A value with a bit outside its members is out of
	// range.
	flagEnum
	// intFlagEnum gives values as flagEnum does, and keeps every integer as
	// a value, so no value is out of its range.
	intFlagEnum
	// strEnum gives the name of the member in lower case.
	strEnum
	// hookedEnum chooses the values of auto through its own hook, which the
	// rules do not run.
	hookedEnum
)

// member is the value of one member of an enum: a text, an integer, or an
// unknown value. variant is the variant of the member, text is the text or
// the decimal digits of an integer, and number is the integer where it fits
// 64 bits.
type member struct {
	name    string
	text    string
	variant *node.EnumVariant
	number  int64
	known   bool
	isText  bool
	fits    bool
}

// EnumOf projects an enum of Python. An enum whose every member has a text,
// such as a StrEnum, is value-formed, and the text of a member is its value.
// Every other enum is identifier-formed, and the text of a member is its
// name. EnumOf computes the value of a member as Python 3.13 and later do:
//
//   - A member of an integer literal or of a string literal has that value.
//   - auto() gives the highest integer value so far plus one, and 1 to the
//     first member. In a Flag and an IntFlag it gives the next power of two
//     above the highest value, and in a StrEnum the name of the member in
//     lower case.
//   - An enum whose class declares _generate_next_value_ chooses the values
//     of auto itself, and any other expression has a value when the module
//     runs. Such a member neither repeats nor bounds another member.
//
// The bases of the stamp python.bases decide the class of the enum, through
// an enum base that the module declares as well. EnumOf also sets these
// fields of the returned [rules.EnumInfo]:
//
//   - Zero is the first member of the value 0, or of the empty text in a
//     value-formed enum.
//   - Duplicate is the earlier member at the first repetition of a value,
//     which Python makes an alias of the earlier member.
//   - OutOfRange is the value after the highest value converted to the
//     enum, and the next power of two above it in a Flag. In a value-formed
//     enum it is the empty text, where no member has that text. An IntFlag
//     keeps every integer, so it has none.
//   - Foreign is the sorted list of the packages other than the package of
//     the enum that declare a member.
//
// A nil enum returns an identifier-formed info without members.
//
// # Allocation contract
//
// EnumOf allocates the list of members, the list of variants and the value
// out of range. The value costs the reference to the enum, the conversion
// and the decimal text of its number. A text with an escape allocates its
// content, a value of auto allocates its text, and a foreign package grows
// the list of foreign packages. The read of the bases of an enum allocates
// the copy of the list and the box of the copy. The reads of the bases and
// of the scope also allocate what the view allocates to record them.
func (r Rules) EnumOf(e *node.Enum, v rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	kind := r.kindOf(e, v, 0)
	ms := r.members(e, kind, v)
	textual := len(ms) > 0 && !slices.ContainsFunc(ms, func(m member) bool { return !m.known || !m.isText })
	if textual {
		info.Form = rules.EnumValue
	}
	info.Variants = make([]rules.VariantText, 0, len(ms))
	var highest int64
	ranged, empty := false, false
	for i, m := range ms {
		text := m.name
		if textual {
			text = m.text
		}
		variant := rules.VariantText{Name: m.name, Text: emit.Literal(emit.LiteralString, text)}
		info.Variants = append(info.Variants, variant)
		if !m.known {
			continue
		}
		first := slices.IndexFunc(ms[:i], func(earlier member) bool {
			return earlier.known && earlier.isText == m.isText && earlier.text == m.text
		})
		if first >= 0 && info.Duplicate == "" {
			info.Duplicate = ms[first].name
		}
		switch {
		case m.isText && m.text == "":
			empty = true
			info.Zero = cmp.Or(info.Zero, m.name)
		case m.isText:
		case m.text == zeroDigits:
			info.Zero = cmp.Or(info.Zero, m.name)
			fallthrough
		default:
			if m.fits && (!ranged || m.number > highest) {
				highest, ranged = m.number, true
			}
		}
	}
	for _, variant := range e.Variants {
		if variant != nil && !variant.ID.IsZero() && variant.ID.Package != e.ID.Package &&
			!slices.Contains(info.Foreign, variant.ID.Package) {

			info.Foreign = append(info.Foreign, variant.ID.Package)
		}
	}
	slices.Sort(info.Foreign)
	t := rules.EmitRef(&node.TypeRef{Spelling: e.Name, Target: e.ID})
	switch {
	case textual && !empty:
		info.OutOfRange = emit.Conversion(t, emit.Literal(emit.LiteralString, ""))
	case textual, !ranged, kind == intFlagEnum:
	case kind == flagEnum:
		next := strconv.FormatInt(nextFlag(highest), decimalBase)
		info.OutOfRange = emit.Conversion(t, emit.Number(emit.LiteralInt, next, 0))
	default:
		next := strconv.FormatInt(highest+1, decimalBase)
		info.OutOfRange = emit.Conversion(t, emit.Number(emit.LiteralInt, next, 0))
	}
	return info
}

// members returns the members of the enum e in declaration order, each with
// its value, where kind decides the values of auto. It leaves out a nil
// variant.
func (Rules) members(e *node.Enum, kind enumKind, v rules.View) []member {
	scope := v.ScopeOf(e.ID)
	pkg, _ := v.PackageOf(e.ID)
	out := make([]member, 0, len(e.Variants))
	var highest int64
	ranged := false
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		m := member{name: variant.Name, variant: variant}
		callee, called := strings.CutSuffix(strings.TrimSpace(variant.Value), callParens)
		lit, scanned := scanLiteral(variant.Value)
		module, name := "", ""
		if called {
			module, name = canonical(pkg, scope.File, callee)
		}
		switch {
		case module == enumModule && name == autoFunction:
			m = autoMember(m, kind, highest, ranged)
		case scanned && lit.kind == literalInt:
			m.known, m.text = true, lit.text
			n, err := strconv.ParseInt(lit.text, decimalBase, 64)
			m.number, m.fits = n, err == nil
		case scanned && lit.kind == literalString:
			m.known, m.isText, m.text = true, true, lit.text
		}
		if m.known && m.fits && (!ranged || m.number > highest) {
			highest, ranged = m.number, true
		}
		out = append(out, m)
	}
	return out
}

// kindOf returns the class of the enum e: the class of the first of its
// bases in the stamp python.bases that decides one. A base of the module
// enum decides by its name, and a base that the module declares decides by
// its own bases, to the default depth of the kernel. An enum that declares
// the hook of auto chooses its values itself.
func (r Rules) kindOf(e *node.Enum, v rules.View, depth int) enumKind {
	if slices.ContainsFunc(e.Methods, func(m *node.Method) bool { return m != nil && m.Name == nextValueHook }) {
		return hookedEnum
	}
	if depth >= rules.DefaultDepth {
		return plainEnum
	}
	bases, _ := rules.Fact(v, e.ID, meta.Named[[]string](python.BasesKey))
	scope := v.ScopeOf(e.ID)
	pkg, _ := v.PackageOf(e.ID)
	for _, base := range bases {
		module, name := canonical(pkg, scope.File, base)
		switch {
		case module == enumModule && name == strEnumClass:
			return strEnum
		case module == enumModule && name == flagClass:
			return flagEnum
		case module == enumModule && name == intFlagClass:
			return intFlagEnum
		case module == e.ID.Package:
			sym, _ := v.Lookup(symbol.Identity{Lang: python.Lang, Package: module, Name: name, Kind: symbol.KindEnum})
			if parent, isEnum := sym.(*node.Enum); isEnum {
				if kind := r.kindOf(parent, v, depth+1); kind != plainEnum {
					return kind
				}
			}
		}
	}
	return plainEnum
}

// autoMember returns the member m with the value that auto gives it in an
// enum of the kind, after the highest integer value so far, where ranged
// reports one. A member of an enum that chooses the values of auto itself
// keeps its unknown value.
func autoMember(m member, kind enumKind, highest int64, ranged bool) member {
	switch kind {
	case hookedEnum:
		return m
	case strEnum:
		m.known, m.isText, m.text = true, true, strings.ToLower(m.name)
		return m
	case plainEnum, flagEnum, intFlagEnum:
	}
	m.known, m.fits = true, true
	switch {
	case !ranged:
		m.number = 1
	case kind == flagEnum || kind == intFlagEnum:
		m.number = nextFlag(highest)
	default:
		m.number = highest + 1
	}
	m.text = strconv.FormatInt(m.number, decimalBase)
	return m
}

// nextFlag returns the next power of two above the highest bit of n, and 1
// for an n below 1.
func nextFlag(n int64) int64 {
	if n < 1 {
		return 1
	}
	return 1 << bits.Len64(uint64(n))
}
