// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The underscores of Python's names. One leading underscore marks a name
// that is not public, and two mark a private member, which Python mangles.
// A trailing underscore keeps a name apart from a keyword. A special name
// of the language starts and ends with two underscores, as __init__ does.
const (
	underscore = "_"
	mangled    = "__"
)

// debugName is the one name that Python binds nowhere. An assignment, a
// parameter and a declaration of __debug__ are syntax errors.
const debugName = "__debug__"

// variadicMark is the mark of a variadic type parameter, which the model
// keeps before its name: one star for a type variable tuple, as *Ts, and
// two for a parameter specification, as **P.
const variadicMark = "*"

// varianceSuffixes are the suffixes that PEP 8 adds to the name of a
// covariant and of a contravariant type variable, as in T_co and T_contra.
var varianceSuffixes = []string{"_co", "_contra"}

// caser is the case conversion of Python. PEP 8 writes an acronym in
// CapWords in capitals, as HTTPServerError, and many sources write it as a
// word, as HttpClient. The conversion recognises no initialism, so a name
// in either style keeps its spelling.
var caser = naming.New()

// keywords are the keywords of Python 3.15, which no name can be. A soft
// keyword, such as type, match, case, lazy and _, is a keyword in its own
// statement alone, so a name can be one.
var keywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true, "assert": true, "async": true,
	"await": true, "break": true, "class": true, "continue": true, "def": true, "del": true, "elif": true,
	"else": true, "except": true, "finally": true, "for": true, "from": true, "global": true, "if": true,
	"import": true, "in": true, "is": true, "lambda": true, "nonlocal": true, "not": true, "or": true,
	"pass": true, "raise": true, "return": true, "try": true, "while": true, "with": true, "yield": true,
}

// The characters of the identifier classes of Unicode that a Python
// identifier refuses. Python defines an identifier by XID_Start and
// XID_Continue, which close the classes under the normal form NFKC. A
// character of nfkcRefused starts and continues no identifier, and a
// character of nfkcNotStart continues an identifier alone.
var (
	nfkcRefused = &unicode.RangeTable{R16: []unicode.Range16{
		{Lo: 0x037a, Hi: 0x037a, Stride: 1},
		{Lo: 0x309b, Hi: 0x309c, Stride: 1},
		{Lo: 0xfc5e, Hi: 0xfc63, Stride: 1},
		{Lo: 0xfdfa, Hi: 0xfdfb, Stride: 1},
		{Lo: 0xfe70, Hi: 0xfe7e, Stride: 2},
	}}
	nfkcNotStart = &unicode.RangeTable{R16: []unicode.Range16{
		{Lo: 0x0e33, Hi: 0x0e33, Stride: 1},
		{Lo: 0x0eb3, Hi: 0x0eb3, Stride: 1},
		{Lo: 0xff9e, Hi: 0xff9f, Stride: 1},
	}}
)

// Name spells one declared name in the conventions of PEP 8:
//
//   - a class, a Protocol, an enum, an alias, a sum, a variant of a sum and
//     a type parameter take CapWords. A type parameter keeps the suffix _co
//     or _contra that PEP 8 adds to a covariant and to a contravariant type
//     variable, as T_co does;
//   - a constant and a member of an enum take UPPER_SNAKE_CASE;
//   - a variable takes snake_case, and a variable in CapWords or in
//     UPPER_SNAKE_CASE keeps its spelling, because an assignment declares a
//     type alias, a type variable and a constant of Python in those styles;
//   - a function, a method, a field, a parameter and a result take
//     snake_case.
//
// The conversion recognises no initialism, so http_server becomes
// HttpServer, and a name in its style, such as HTTPServerError, keeps its
// spelling. A name that starts and ends with two underscores, such as
// __init__, keeps its spelling, and so does a name of underscores alone.
//
// The visibility sets the leading underscores of a name. A public member of
// a class has none. A protected, a package and an internal member has one,
// and a private member has two, which Python mangles. A declaration at file
// level that is not public has one, and keeps two where it has two. A
// public declaration at file level keeps the underscores that it has,
// because a module that lists it in __all__ exports it. A parameter, a
// result, a type parameter and a variant keep their underscores, because
// they have no visibility. A variadic type parameter keeps its stars before
// its name, and its name takes CapWords after them, as *Ts and **P do.
//
// The trailing underscores of a name remain. A spelling that is a keyword of
// Python takes a trailing underscore, as PEP 8 recommends, so class becomes
// class_. A soft keyword, such as type or match, keeps its spelling.
//
// A name that is not an identifier, such as the wire name content-type,
// passes through unchanged on a field and on a member of an enum, so a
// convention cannot rename what a consumer matches by string. The backend
// writes it in the functional form of a TypedDict and of an enum, and
// refuses it elsewhere. Name returns an error for a name that is not an
// identifier on any other kind, and for a spelling that is not an
// identifier, such as a public member _1st. It returns an error for
// __debug__, which Python binds nowhere.
//
// # Allocation contract
//
// Name allocates a converted spelling, and the spelling with its
// underscores or its stars where they change, one allocation each. A name
// that the convention spells as it is allocates nothing. A refusal
// allocates its error.
func Name(host, kind symbol.Kind, vis symbol.Visibility, name string) (string, error) {
	return spellName(host, kind, vis, name, false)
}

// Member spells the name of a member in the conventions of PEP 8, as [Name]
// spells it. A field at the level of the type or an immutable field keeps
// its spelling in UPPER_SNAKE_CASE, because Python writes a constant of a
// class in capitals, as MAX_LINES: ClassVar[int] = 100 and LIMIT: Final = 10
// do. Its errors and its allocations are those of Name.
func Member(m plugin.Member, name string) (string, error) {
	constant := m.Kind == symbol.KindField &&
		(m.Level == symbol.LevelType || m.Mutability == symbol.MutabilityImmutable)
	return spellName(m.Host, m.Kind, m.Visibility, name, constant)
}

// spellName spells one declared name by the rules of [Name]. constant
// reports a field that is a constant of its class, which keeps its spelling
// in UPPER_SNAKE_CASE.
func spellName(host, kind symbol.Kind, vis symbol.Visibility, name string, constant bool) (string, error) {
	bare := strings.TrimPrefix(strings.TrimPrefix(name, variadicMark), variadicMark)
	if kind == symbol.KindTypeParam && bare != name && IsIdentifier(bare) {
		spelt, err := spellName(host, kind, vis, bare, false)
		switch {
		case err != nil:
			return "", err
		case spelt == bare:
			return name, nil
		}
		return name[:len(name)-len(bare)] + spelt, nil
	}
	identifier := IsIdentifier(name)
	switch {
	case !identifier && (kind == symbol.KindField || kind == symbol.KindEnumVariant):
		return name, nil
	case !identifier:
		return "", fmt.Errorf(refusalPrefix+"%q is not an identifier, and the name of a declaration of the kind %s "+
			"has no other form", name, kind)
	case name == debugName:
		return "", fmt.Errorf(refusalPrefix+"no declaration can bind %s", debugName)
	}
	trimmed := strings.TrimLeft(name, underscore)
	core := strings.TrimRight(trimmed, underscore)
	special := len(name) > 2*len(mangled) && strings.HasPrefix(name, mangled) && strings.HasSuffix(name, mangled)
	if core == "" || special {
		return name, nil
	}
	lead, trail := name[:len(name)-len(trimmed)], trimmed[len(core):]
	var spelt string
	switch kind {
	case symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindAlias, symbol.KindSum,
		symbol.KindSumVariant:
		spelt = caser.Pascal(core)
	case symbol.KindTypeParam:
		stem, suffix := core, ""
		for _, s := range varianceSuffixes {
			if cut, found := strings.CutSuffix(core, s); found {
				stem, suffix = cut, s
				break
			}
		}
		spelt = caser.Pascal(stem) + suffix
	case symbol.KindConstant, symbol.KindEnumVariant:
		spelt = caser.ScreamingSnake(core)
	case symbol.KindVariable:
		first, _ := utf8.DecodeRuneInString(core)
		spelt = core
		if !unicode.IsUpper(first) ||
			strings.Contains(core, underscore) && strings.ContainsFunc(core, unicode.IsLower) {

			spelt = caser.Snake(core)
		}
	case symbol.KindField:
		spelt = core
		if !constant || strings.ContainsFunc(core, unicode.IsLower) {
			spelt = caser.Snake(core)
		}
	default:
		spelt = caser.Snake(core)
	}
	prefix := underscores(host, kind, vis, lead)
	switch {
	case prefix == "" && trail == "" && keywords[spelt]:
		return spelt + underscore, nil
	case prefix == lead && spelt == core:
		return name, nil
	}
	out := prefix + spelt + trail
	if !IsIdentifier(out) {
		return "", fmt.Errorf(refusalPrefix+"the convention writes %s as %s, which is not an identifier", name, out)
	}
	return out, nil
}

// IsIdentifier reports whether name is an identifier of Python: a character
// of XID_Start or an underscore, followed by characters of XID_Continue, as
// the lexical analysis of the Python language reference defines it. The
// check reads the categories and the properties of the Unicode tables of
// Go, which list Unicode 17.0.0, the version of CPython 3.15. It refuses the
// characters that XID_Start and XID_Continue leave out under NFKC. An ASCII
// identifier has letters, digits and underscores, and does not start with a
// digit. An empty name is no identifier. IsIdentifier does not allocate.
func IsIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r < utf8.RuneSelf {
			if r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (i == 0 || r < '0' || r > '9') {
				return false
			}
			continue
		}
		switch {
		case unicode.In(r, unicode.Pattern_Syntax, unicode.Pattern_White_Space, nfkcRefused):
			return false
		case unicode.IsLetter(r) || unicode.In(r, unicode.Nl, unicode.Other_ID_Start):
			if i == 0 && unicode.Is(nfkcNotStart, r) {
				return false
			}
		case i == 0 || !unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue):
			return false
		}
	}
	return true
}

// IsKeyword reports whether name is a keyword of Python 3.15, which no
// declaration can take as its name. A soft keyword, such as type, match,
// case, lazy and _, is a keyword in its own statement alone, so IsKeyword
// reports false for it. IsKeyword does not allocate.
func IsKeyword(name string) bool { return keywords[name] }

// underscores returns the leading underscores of a name at the visibility
// vis. kind is the kind of the declaration that has the name, host is the
// kind of the declaration that declares it, and lead are the leading
// underscores of the name as written. A kind without
// visibility keeps lead, and so does a declaration at file level that is
// public or has lead. Any other declaration at file level, and a protected,
// a package and an internal member, has one underscore. A public member has
// none, and a private member has two.
func underscores(host, kind symbol.Kind, vis symbol.Visibility, lead string) string {
	public := vis == symbol.VisibilityUnknown || vis == symbol.VisibilityPublic
	switch {
	case kind == symbol.KindParam || kind == symbol.KindReturn || kind == symbol.KindTypeParam ||
		kind == symbol.KindEnumVariant || kind == symbol.KindSumVariant:
		return lead
	case host == symbol.KindInvalid && (public || lead != ""):
		return lead
	case host == symbol.KindInvalid, !public && vis != symbol.VisibilityPrivate:
		return underscore
	case public:
		return ""
	default:
		return mangled
	}
}
