// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The wire numbers of a field. protobuf takes a number from 1 to 2^29 - 1,
// and it reserves 19,000 to 19,999 for its implementation.
const (
	minNumber     = 1
	maxNumber     = 1<<29 - 1
	reservedFirst = 19_000
	reservedLast  = 19_999
)

// The option of an enum whose values can share a number, and its value
// that allows it.
const (
	allowAliasOption = "allow_alias"
	allowAliasValue  = "true"
)

// The syntax of the value of an enum value: a minus sign, and the prefixes
// of a hexadecimal and of an octal number. A plus sign is no syntax of
// protobuf, and the parse of the digits refuses it.
const (
	minusSign   = "-"
	plusSign    = "+"
	hexPrefix   = "0x"
	upperHex    = "0X"
	octalPrefix = '0'
)

// The fields that the lowering names: the field of a variant message
// without a name, after which its position follows.
const fieldName = "field"

// localMembers is the number of members of a message that the checks of
// its names and numbers keep on the stack.
const localMembers = 16

// localValues is the number of values of an enum that the check of its
// numbers keeps on the stack.
const localValues = 32

// caser converts the names of the messages that the lowering declares: the
// message of a variant, a wrapper, a request and a response. It converts as
// [spell.Name] does, without initialisms, so the respell keeps each name.
var caser = naming.New()

// Lower rewrites one declaration into the shapes that protobuf writes, in
// place and before any name respells. It refuses a construct that protobuf
// has no form of, and the settle withholds the declaration. The rewrites
// are these:
//
//   - a sum at file level becomes a message with the name, the
//     documentation and the annotations of the sum, and one oneof of the
//     variants of the sum, because a protobuf schema writes a sum type as a
//     message with a oneof;
//   - a variant without a field becomes a member of the type
//     google.protobuf.Empty. A variant with two or more fields becomes a
//     member of a message that the lowering declares in the message of the
//     oneof, with the name of the variant in Pascal case and the fields of
//     the variant. Both take the number of the marker protobuf:number of
//     the variant;
//   - a field of a form that protobuf does not write becomes a field of a
//     wrapper message, as the protobuf ecosystem writes it;
//   - an rpc takes one request and returns one response, through a message
//     that the lowering declares beside the service where the method does
//     not take or return one message.
//
// Each rewrite states its result in the shapes of protobuf, so a second
// lowering changes nothing. A message that the lowering declares numbers
// each field without a number by its position.
//
// Lower keeps a declaration with type parameters as it is, because the
// coverage rejects it at the render. It keeps a function, a method at file
// level, an alias, a constant and a variable as they are, because the
// render refuses their kinds.
//
// Error modes, each a refusal of the declaration:
//   - an annotation that its kind does not take;
//   - a field of a message without one tag entry number, or with a number
//     outside 1 to 536,870,911 or from 19,000 to 19,999;
//   - two members of a message with one number or one name;
//   - a read-only field, a wire name that is not an identifier, and a field
//     at file level that extends no message;
//   - a field among the nested declarations of a message that extends no
//     message, an extension among its fields, and a nested declaration of
//     a kind that a message does not nest;
//   - an enum without a value, a value without a number in decimal,
//     hexadecimal or octal or outside the range of an int32, and two values
//     with one number where the enum does not state allow_alias;
//   - a oneof without a member, a variant with one field and the marker
//     protobuf:number, and a variant without exactly one field and without
//     the marker;
//   - a service with fields or with two rpcs of one name, a stream beside
//     another parameter or result, and a parameter with a label;
//   - the deleter of a property, through [lowering.DeleteAccessor], because
//     a schema has no properties.
//
// # Allocation contract
//
// A declaration that the lowering keeps as it is allocates nothing. A
// rewrite allocates the declarations that it writes. A refusal allocates
// its error.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	if err := lowering.DeleteAccessor(protobuf.Lang, s); err != nil {
		return nil, err
	}
	switch d := s.(type) {
	case *emit.File:
		return nil, checkAnnotations(symbol.KindFile, headerName, d.Annotations)
	case *emit.Struct:
		if len(d.TypeParams) > 0 {
			return nil, nil
		}
		return nil, lowerMessage(d)
	case *emit.Sum:
		if len(d.TypeParams) > 0 {
			return nil, nil
		}
		m := &emit.Struct{
			Origin: d.Origin, Doc: d.Doc, Comment: d.Comment, Name: d.Name,
			Visibility: d.Visibility, Annotations: d.Annotations,
		}
		m.Types.Append(&emit.Sum{Name: d.Name, Variants: d.Variants, Methods: d.Methods})
		if err := lowerMessage(m); err != nil {
			return nil, err
		}
		return []symbol.Symbol{m}, nil
	case *emit.Enum:
		return nil, lowerEnum(d)
	case *emit.Interface:
		if len(d.TypeParams) > 0 {
			return nil, nil
		}
		return lowerService(d)
	case *emit.Field:
		return lowerExtension(d)
	default:
		return nil, nil
	}
}

// lowerMessage lowers the message s and its nested declarations: its
// fields, its nested messages and enums, its oneofs and its extensions. The
// wrapper messages and the variant messages that a member needs are nested
// declarations of s.
//
// The fields, the oneofs, the members of the oneofs, the nested
// declarations and the extensions of a message share its names, and its
// fields and the members of its oneofs share its numbers. The closure
// member records them in two slices, which are on the stack for a message
// of up to 16 members.
func lowerMessage(s *emit.Struct) error {
	if err := checkAnnotations(symbol.KindStruct, s.Name, s.Annotations); err != nil {
		return err
	}
	var (
		nameStack   [localMembers]string
		numberStack [localMembers]numbered
	)
	names, numbers := nameStack[:0], numberStack[:0]
	member := func(name string, number uint32) error {
		if slices.Contains(names, name) {
			return fmt.Errorf(refusalPrefix+"%s declares two members named %s", s.Name, name)
		}
		names = append(names, name)
		if number == 0 {
			return nil
		}
		if i := slices.IndexFunc(numbers, func(n numbered) bool { return n.number == number }); i >= 0 {
			return fmt.Errorf(refusalPrefix+"%s and %s of %s take the wire number %d",
				numbers[i].name, name, s.Name, number)
		}
		numbers = append(numbers, numbered{name: name, number: number})
		return nil
	}
	add := func(m *emit.Struct) { s.Types.Append(m) }
	for _, f := range s.Fields.Items() {
		if f.Receives != nil {
			return fmt.Errorf(refusalPrefix+"an extension is a nested declaration of a message, "+
				"and %s lists %s among its fields", s.Name, f.Name)
		}
		n, err := lowerField(f.Name, f, add)
		if err != nil {
			return err
		}
		if err := member(f.Name, n); err != nil {
			return err
		}
	}
	// The loop reads the length of the slot on each pass, so it also lowers
	// and names each wrapper message that a member declares.
	for i := 0; i < s.Types.Len(); i++ {
		if err := lowerNested(s, s.Types.Items()[i], add, member); err != nil {
			return err
		}
	}
	return nil
}

// lowerNested lowers the nested declaration t of the message s, and records
// its names and its numbers through member. A oneof records its name, and
// each member records its name and its number. A nested message, an enum
// and an extension record their names alone, because the number of an
// extension is a number of the message that it extends.
func lowerNested(s *emit.Struct, t symbol.Symbol, add func(*emit.Struct), member func(string, uint32) error) error {
	switch n := t.(type) {
	case *emit.Struct:
		if err := lowerMessage(n); err != nil {
			return err
		}
		return member(n.Name, 0)
	case *emit.Enum:
		if err := lowerEnum(n); err != nil {
			return err
		}
		return member(n.Name, 0)
	case *emit.Sum:
		if err := member(n.Name, 0); err != nil {
			return err
		}
		return lowerOneof(n, add, member)
	case *emit.Field:
		if n.Receives == nil {
			return fmt.Errorf(refusalPrefix+"a field of a message is one of its fields, "+
				"and %s nests %s among its nested declarations", s.Name, n.Name)
		}
		if _, err := lowerField(n.Name, n, add); err != nil {
			return err
		}
		return member(n.Name, 0)
	default:
		return fmt.Errorf(refusalPrefix+"a message nests messages, enums, oneofs and extensions, "+
			"and %s nests a declaration of the kind %s", s.Name, t.Kind())
	}
}

// lowerField lowers the field f, whose member takes the name name, and
// returns its wire number. A wrapper message that its form needs is
// declared through add.
func lowerField(name string, f *emit.Field, add func(*emit.Struct)) (uint32, error) {
	if err := checkAnnotations(symbol.KindField, name, f.Annotations); err != nil {
		return 0, err
	}
	if f.Mutability == symbol.MutabilityImmutable {
		return 0, fmt.Errorf(refusalPrefix+"a message has no read-only field, and %s is read-only", name)
	}
	if f.Wire && !naming.IsIdentifier(name) {
		return 0, fmt.Errorf(refusalPrefix+"a field name is an identifier, and the wire name %q is not one", name)
	}
	n, err := fieldNumber(name, f)
	if err != nil {
		return 0, err
	}
	f.Type = shape(name, f.Type, add)
	return n, nil
}

// lowerExtension lowers a field at file level, which is an extension of
// the message that its Receives names. It returns the field and the
// wrapper messages that its form needs, which take the origin of the field
// at file level, and nil where the form needs none.
func lowerExtension(f *emit.Field) ([]symbol.Symbol, error) {
	if f.Receives == nil {
		return nil, fmt.Errorf(refusalPrefix+"a file declares no field of its own, and %s extends no message", f.Name)
	}
	var wrappers []symbol.Symbol
	_, err := lowerField(f.Name, f, func(m *emit.Struct) {
		m.Origin = f.Origin
		wrappers = append(wrappers, m)
	})
	if err != nil {
		return nil, err
	}
	if len(wrappers) == 0 {
		return nil, nil
	}
	return slices.Insert(wrappers, 0, symbol.Symbol(f)), nil
}

// lowerOneof lowers the oneof o, so each of its variants has one member
// field, and records the name and the number of each member through
// member. The message of a member is declared through add.
func lowerOneof(o *emit.Sum, add func(*emit.Struct), member func(string, uint32) error) error {
	if err := checkAnnotations(symbol.KindSum, o.Name, o.Annotations); err != nil {
		return err
	}
	if o.Variants.Len() == 0 {
		return fmt.Errorf(refusalPrefix+"a oneof declares at least one member, and %s declares none", o.Name)
	}
	for _, v := range o.Variants.Items() {
		number, err := lowerVariant(v, add)
		if err != nil {
			return err
		}
		if err := member(v.Name, number); err != nil {
			return err
		}
	}
	return nil
}

// lowerVariant lowers the variant v into one member field, which takes the
// name of the variant, and returns the wire number of the member. The field
// of a variant with one field is the member. A variant without exactly one
// field takes a new member of the number of its marker protobuf:number,
// which the lowering removes.
func lowerVariant(v *emit.SumVariant, add func(*emit.Struct)) (uint32, error) {
	if err := checkAnnotations(symbol.KindSumVariant, v.Name, v.Annotations); err != nil {
		return 0, err
	}
	marker := slices.IndexFunc(v.Annotations, func(a symbol.Annotation) bool {
		return a.Name == string(protobuf.NumberMarker)
	})
	fields := v.Fields.Items()
	if len(fields) == 1 {
		if marker >= 0 {
			return 0, fmt.Errorf(refusalPrefix+"the marker %s numbers a variant without exactly one field, "+
				"and %s has one", protobuf.NumberMarker, v.Name)
		}
		return lowerMember(v.Name, fields[0], add)
	}
	if marker < 0 {
		return 0, fmt.Errorf(refusalPrefix+"a variant without exactly one field takes the marker %s, "+
			"and %s states none", protobuf.NumberMarker, v.Name)
	}
	number := v.Annotations[marker].Args[0]
	n, err := wireNumber(v.Name, number)
	if err != nil {
		return 0, err
	}
	member := &emit.Field{Name: v.Name, Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: number})}
	if len(fields) == 0 {
		member.Type = &emit.TypeRef{Spelling: protobuf.TypeEmpty, Package: protobuf.EmptyFile}
	} else {
		m := &emit.Struct{Name: caser.Pascal(v.Name)}
		for i, f := range fields {
			if f.Name == "" {
				f.Name = fieldName + positionSep + strconv.Itoa(i+1)
			}
			if f.Tags.Len() == 0 {
				f.Tags = emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: strconv.Itoa(i + 1)})
			}
			m.Fields.Append(f)
		}
		if err := lowerMessage(m); err != nil {
			return 0, err
		}
		add(m)
		member.Type = &emit.TypeRef{Spelling: m.Name}
	}
	v.Fields.Replace(member)
	v.Annotations = slices.Delete(slices.Clone(v.Annotations), marker, marker+1)
	return n, nil
}

// lowerMember lowers the field f of a variant as the member name of a
// oneof, and returns its wire number. A member has presence through its
// oneof, so the member of an optional writes the type of the optional. A
// member writes no list and no map, so it takes a wrapper message of the
// list or the map, which the lowering declares through add.
func lowerMember(name string, f *emit.Field, add func(*emit.Struct)) (uint32, error) {
	n, err := lowerField(name, f, add)
	if err != nil {
		return 0, err
	}
	if f.Type == nil {
		return n, nil
	}
	switch f.Type.Form {
	case symbol.FormOptional:
		if len(f.Type.Elems) == 1 && f.Type.Elems[0].Form == symbol.FormNamed {
			f.Type = f.Type.Elems[0]
		}
	case symbol.FormList, symbol.FormMap:
		f.Type = wrapped(caser.Pascal(name)+valueSuffix, valuesField, f.Type, add)
	default:
		// A named type is the type of the member.
	}
	return n, nil
}

// lowerEnum lowers the enum e: it checks its values, and refuses two values
// with one number where e does not state allow_alias. The check compares
// each pair of values, and the values of an enum of up to 32 values are on
// the stack.
func lowerEnum(e *emit.Enum) error {
	if err := checkAnnotations(symbol.KindEnum, e.Name, e.Annotations); err != nil {
		return err
	}
	variants := e.Variants.Items()
	if len(variants) == 0 {
		return fmt.Errorf(refusalPrefix+"an enum declares at least one value, and %s declares none", e.Name)
	}
	aliased := slices.ContainsFunc(e.Annotations, func(a symbol.Annotation) bool {
		return a.Name == allowAliasOption && len(a.Args) == 1 && a.Args[0] == allowAliasValue
	})
	var stack [localValues]int32
	values := stack[:0]
	for _, v := range variants {
		if err := checkAnnotations(symbol.KindEnumVariant, v.Name, v.Annotations); err != nil {
			return err
		}
		n, err := enumValue(v)
		if err != nil {
			return err
		}
		if j := slices.Index(values, n); j >= 0 && !aliased {
			return fmt.Errorf(refusalPrefix+"%s and %s of %s take the value %d, and %s does not state %s = %s",
				variants[j].Name, v.Name, e.Name, n, e.Name, allowAliasOption, allowAliasValue)
		}
		values = append(values, n)
	}
	return nil
}

// enumValue returns the number of the enum value v, which protobuf writes
// in decimal, in hexadecimal after 0x or in octal after 0, with a minus
// sign before a negative number.
func enumValue(v *emit.EnumVariant) (int32, error) {
	if v.Value == "" {
		return 0, fmt.Errorf(refusalPrefix+"an enum value takes a number, and %s states none", v.Name)
	}
	digits, negative := strings.CutPrefix(v.Value, minusSign)
	base := 10
	switch {
	case len(digits) > len(hexPrefix) && (strings.HasPrefix(digits, hexPrefix) || strings.HasPrefix(digits, upperHex)):
		base, digits = 16, digits[len(hexPrefix):]
	case len(digits) > 1 && digits[0] == octalPrefix:
		base, digits = 8, digits[1:]
	}
	n, err := strconv.ParseInt(digits, base, 64)
	switch {
	case strings.HasPrefix(digits, plusSign) || strings.HasPrefix(digits, minusSign):
		err = strconv.ErrSyntax
	case negative:
		n = -n
	}
	switch {
	case err != nil && !errors.Is(err, strconv.ErrRange):
		return 0, fmt.Errorf(refusalPrefix+"an enum value is a number in decimal, hexadecimal or octal, "+
			"and %s states %s", v.Name, v.Value)
	case err != nil || n < math.MinInt32 || n > math.MaxInt32:
		return 0, fmt.Errorf(refusalPrefix+"an enum value is an int32, and %s states %s", v.Name, v.Value)
	}
	return int32(n), nil
}

// fieldNumber returns the wire number of the field f of the member name:
// the value of its one tag entry number.
func fieldNumber(name string, f *emit.Field) (uint32, error) {
	entries := f.Tags.Items()
	switch {
	case len(entries) == 0:
		return 0, fmt.Errorf(refusalPrefix+"a field of a message takes a number, and %s states none", name)
	case len(entries) > 1:
		return 0, fmt.Errorf(refusalPrefix+"a field takes one tag entry, and %s states %d", name, len(entries))
	case entries[0].Key != protobuf.NumberTag:
		return 0, fmt.Errorf(refusalPrefix+"a field takes the tag entry %s, and %s states the entry %s",
			protobuf.NumberTag, name, entries[0].Key)
	}
	return wireNumber(name, entries[0].Value)
}

// wireNumber returns the wire number that value states in decimal for the
// member name.
func wireNumber(name, value string) (uint32, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	switch {
	case err != nil:
		return 0, fmt.Errorf(refusalPrefix+"a wire number is a decimal number, and %s states %q", name, value)
	case n < minNumber || n > maxNumber:
		return 0, fmt.Errorf(refusalPrefix+"a wire number is from 1 to 536870911, and %s states %d", name, n)
	case n >= reservedFirst && n <= reservedLast:
		return 0, fmt.Errorf(refusalPrefix+"protobuf reserves the wire numbers 19000 to 19999, "+
			"and %s states %d", name, n)
	}
	return uint32(n), nil
}

// numbered is the wire number of one member of a message.
type numbered struct {
	name   string
	number uint32
}
