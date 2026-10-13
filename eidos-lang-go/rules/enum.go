// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// platformBits is the width that the range of an integer of the platform
// width takes. Every platform of Go holds 32 bits in an int.
const platformBits = 32

// singleBits is the width of float32. A float of any other width has the
// precision of float64.
const singleBits = 32

// The parts of the texts of an enumeration: the underscore that can follow
// the name of the type in the name of a variant, the exact value of the
// empty string, and the mark that the text probe repeats.
const (
	nameSep    = "_"
	emptyExact = `""`
	probeMark  = "?"
)

// constValue is the handle of the exact constant values that the Go
// frontend stamps. A read resolves the handle in the registry of the
// facts of the view, and a composition that did not register the key
// reads no value.
var constValue = meta.Named[string](golang.ConstValueKey)

// EnumOf projects a Go enumeration, which is a defined type with typed
// constants. EnumOf derives the projection from the underlying type, which
// it resolves to a builtin through the defined types and the enumerations
// of the view. Where the underlying type does not resolve, EnumOf reads its
// kind from the first exact value, because Go gives a constant the kind of
// its type. A quoted value is a string, true and false are a bool, and any
// other value is a number of an unknown width.
//
//   - The form is EnumValue for a string, and EnumIdentifier otherwise.
//   - The text of a variant of a string is its unquoted exact value, and
//     the zero value where the frontend stamped none. The text of any other
//     variant is its name without the name of the type and an underscore
//     after it, where an upper-case letter, a digit or the underscore
//     follows the name of the type. Otherwise it is the whole name.
//   - The zero is the first variant whose exact value is the zero of the
//     underlying type: 0, "" or false.
//   - The duplicate is the name of the first variant whose value a later
//     variant repeats.
//   - The foreign packages are the packages other than the package of the
//     type that declare a variant.
//
// The out-of-range value is a value of the underlying type that no variant
// has:
//
//   - a string takes "" where no variant has it, and otherwise a run of ?
//     one byte longer than the longest text;
//   - a bool takes the truth value that no variant has;
//   - an integer takes one past the largest value where the underlying
//     type holds it, and one below the smallest value otherwise;
//   - a float takes the largest value plus one at its precision, or the
//     next value of that precision where the sum rounds back. Where that
//     value is infinite, the same rule applies below the smallest value.
//
// A set at both ends of its type, a variant without an exact value and a
// number of an unknown width have no out-of-range value.
//
// EnumOf compares integers as int64. An integer that int64 does not hold,
// such as a uint64 value from 2^63 up, takes no part in the range. It
// reads a float as a rational number, which reads the fractions that the
// frontend stamps, such as 3/2.
//
// # Allocation contract
//
// EnumOf allocates the variant list, sized to the enumeration, and the
// out-of-range value: the reference to the type and its conversion. A
// float allocates the rational number of each value and the decimal text
// of the out-of-range value, and a text probe of question marks allocates
// its text. A text with an escape allocates its content, and a foreign
// package grows the list of foreign packages.
func (r Rules) EnumOf(e *node.Enum, v rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	shape := r.literalShape(e.Underlying, v, 0)
	sized := shape.Form == symbol.FormScalar
	if !sized && shape.Form != symbol.FormText && shape.Form != symbol.FormBool {
		shape = rules.TypeShape{Form: stampedForm(e, v)}
	}
	textual := shape.Form == symbol.FormText
	if textual {
		info.Form = rules.EnumValue
	}
	zero := sampleZero
	switch shape.Form {
	case symbol.FormText:
		zero = emptyExact
	case symbol.FormBool:
		zero = falseSpelling
	default:
		// A number has the zero 0.
	}
	info.Variants = make([]rules.VariantText, 0, len(e.Variants))
	seen := map[string]string{}
	span := valueRange{complete: true, form: shape.Form, class: shape.Class, bits: shape.Bits}
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		value, stamped := rules.Fact(v, variant.ID, constValue)
		var text emit.Value
		unquoted, err := strconv.Unquote(value)
		switch {
		case !textual:
			text = emit.Literal(emit.LiteralString, identifierText(e.Name, variant.Name))
		case stamped && err == nil:
			text = emit.Literal(emit.LiteralString, unquoted)
		default:
			// A variant of a string without an exact value has no text.
		}
		info.Variants = append(info.Variants, rules.VariantText{Name: variant.Name, Text: text})
		if !variant.ID.IsZero() && variant.ID.Package != e.ID.Package &&
			!slices.Contains(info.Foreign, variant.ID.Package) {

			info.Foreign = append(info.Foreign, variant.ID.Package)
		}
		if !stamped {
			span.complete = false
			continue
		}
		if value == zero && info.Zero == "" {
			info.Zero = variant.Name
		}
		if first, dup := seen[value]; dup && info.Duplicate == "" {
			info.Duplicate = first
		} else if !dup {
			seen[value] = variant.Name
		}
		span.add(value, unquoted)
	}
	slices.Sort(info.Foreign)
	if !span.complete || shape.Form == symbol.FormScalar && !sized {
		return info
	}
	if outside, derived := span.outside(bounds(shape)); derived {
		ref := &node.TypeRef{Spelling: e.Name, Target: e.ID}
		info.OutOfRange = emit.Conversion(rules.EmitRef(ref), outside)
	}
	return info
}

// identifierText returns the text of the variant name of the enumeration
// typeName in the identifier form: name without typeName and an
// underscore after it, where an upper-case letter, a digit or the
// underscore follows typeName in name and text remains. Otherwise it
// returns the whole name. It allocates nothing.
func identifierText(typeName, name string) string {
	rest, cut := strings.CutPrefix(name, typeName)
	if !cut || rest == "" {
		return name
	}
	if next, _ := utf8.DecodeRuneInString(rest); !unicode.IsUpper(next) && !unicode.IsDigit(next) &&
		!strings.HasPrefix(rest, nameSep) {

		return name
	}
	if rest = strings.TrimPrefix(rest, nameSep); rest == "" {
		return name
	}
	return rest
}

// stampedForm returns the form of the underlying type of e from the first
// exact value that the frontend stamped on a variant. A quoted value is a
// string, true and false are a bool, and any other value is a number. It
// returns [symbol.FormNamed], a form without a kind, where no variant has
// an exact value.
func stampedForm(e *node.Enum, v rules.View) symbol.TypeForm {
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		value, stamped := rules.Fact(v, variant.ID, constValue)
		switch {
		case !stamped:
			continue
		case strings.HasPrefix(value, stringQuote):
			return symbol.FormText
		case value == trueSpelling || value == falseSpelling:
			return symbol.FormBool
		default:
			return symbol.FormScalar
		}
	}
	return symbol.FormNamed
}

// valueRange is the range of the exact values of an enumeration, outside
// which the out-of-range value lies.
type valueRange struct {
	// The integers from smallest to largest, or the floats from low to
	// high, are the bounds of a range of numbers.
	smallest, largest int64
	low, high         float64
	// longest is the length of the longest text in bytes.
	longest int
	// bits, form and class are the kind of the values that the range
	// reads.
	bits  int
	form  symbol.TypeForm
	class rules.ScalarClass
	// empty reports that a text is the empty string.
	empty bool
	// complete reports that every variant has an exact value, and ranged
	// that a number joined the range.
	complete, ranged bool
	// sawFalse and sawTrue report the truth values that occur.
	sawFalse, sawTrue bool
}

// add joins the exact value of one variant to the range. text is the
// unquoted value of a string. A number that its class does not read takes
// no part.
func (s *valueRange) add(value, text string) {
	switch {
	case s.form == symbol.FormText:
		s.empty = s.empty || text == ""
		s.longest = max(s.longest, len(text))
	case s.form == symbol.FormBool:
		s.sawTrue = s.sawTrue || value == trueSpelling
		s.sawFalse = s.sawFalse || value == falseSpelling
	case s.class == rules.ScalarFloat:
		rat, read := new(big.Rat).SetString(value)
		if !read {
			return
		}
		f, _ := rat.Float64()
		if !s.ranged || f < s.low {
			s.low = f
		}
		if !s.ranged || f > s.high {
			s.high = f
		}
		s.ranged = true
	default:
		n, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			return
		}
		if !s.ranged || n < s.smallest {
			s.smallest = n
		}
		if !s.ranged || n > s.largest {
			s.largest = n
		}
		s.ranged = true
	}
}

// outside returns a value of the kind of the range that the range does not
// contain, and reports false where the kind holds none. lowest and highest
// are the bounds of an integer kind.
func (s *valueRange) outside(lowest, highest int64) (emit.Value, bool) {
	switch {
	case s.form == symbol.FormText && !s.empty:
		return emit.Literal(emit.LiteralString, ""), true
	case s.form == symbol.FormText:
		return emit.Literal(emit.LiteralString, strings.Repeat(probeMark, s.longest+1)), true
	case s.form == symbol.FormBool && s.sawFalse && s.sawTrue:
		return emit.Value{}, false
	case s.form == symbol.FormBool:
		return emit.Literal(emit.LiteralBool, strconv.FormatBool(s.sawFalse)), true
	case !s.ranged:
		return emit.Value{}, false
	case s.class == rules.ScalarFloat:
		f, past := floatStep(s.high, 1, s.bits)
		if !past {
			f, past = floatStep(s.low, -1, s.bits)
		}
		if !past {
			return emit.Value{}, false
		}
		return emit.Number(emit.LiteralFloat, numeric.Decimal(f, s.bits), s.bits), true
	case s.largest < highest:
		return emit.Literal(emit.LiteralInt, strconv.FormatInt(s.largest+1, 10)), true
	case s.smallest > lowest:
		return emit.Literal(emit.LiteralInt, strconv.FormatInt(s.smallest-1, 10)), true
	default:
		return emit.Value{}, false
	}
}

// floatStep returns the float x plus step at the precision of a float of
// the width bits, or the next value of that precision in the direction of
// step where the sum rounds back to x. It reports false where that value
// is infinite. A width of 32 is the precision of float32, and every other
// width that of float64.
func floatStep(x, step float64, bits int) (float64, bool) {
	next := x + step
	if bits == singleBits {
		narrow := float32(next)
		if narrow == float32(x) {
			narrow = math.Nextafter32(float32(x), float32(math.Inf(int(step))))
		}
		next = float64(narrow)
	} else if next == x {
		next = math.Nextafter(x, math.Inf(int(step)))
	}
	return next, !math.IsInf(next, 0)
}

// bounds returns the smallest and the largest value of an integer of the
// shape s, within the range of int64. An integer of the platform width has
// the range of 32 bits, because Go defines int and uint as 32 or 64 bits
// wide. A float and every shape that is not a scalar have the range of
// int64. bounds allocates nothing.
func bounds(s rules.TypeShape) (int64, int64) {
	if s.Form != symbol.FormScalar || s.Class == rules.ScalarFloat {
		return math.MinInt64, math.MaxInt64
	}
	bits := s.Bits
	if bits == 0 {
		bits = platformBits
	}
	switch {
	case s.Class == rules.ScalarUint && bits >= 64:
		return 0, math.MaxInt64
	case s.Class == rules.ScalarUint:
		return 0, 1<<bits - 1
	case bits >= 64:
		return math.MinInt64, math.MaxInt64
	default:
		return -1 << (bits - 1), 1<<(bits-1) - 1
	}
}
