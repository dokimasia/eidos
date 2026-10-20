// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"cmp"
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// These constants are the modifiers that a line of the surface starts
// with, as indexes of [modifierWords] in the order of the line.
const (
	modStatic      = 0
	modAbstract    = 1
	modFinal       = 2
	modSealed      = 3
	modConst       = 4
	modAsync       = 5
	modReadonly    = 6
	modGet         = 7
	modSet         = 8
	modDelete      = 9
	modConstructor = 10
	modIndexer     = 11
	modCount       = 12
)

// modifierWords are the words of the modifiers, each at the index of its
// modifier.
var modifierWords = [modCount]string{
	"static", "abstract", "final", "sealed", "const", "async", "readonly", "get", "set", "delete", "constructor",
	"indexer",
}

// These constants are the words of a line of the surface: the start of
// the line of a package, the relations of a host, and the marks of a
// parameter, a value and an optional member.
const (
	packageLine    = "package "
	relExtends     = " extends "
	relImplements  = " implements "
	relEmbeds      = " embeds "
	relPermits     = " permits "
	variadicMark   = "..."
	optionalMark   = "?"
	positionalMark = "/"
	labelMark      = ": "
	valueMark      = " = "
)

// Surface returns the public surface of the files of g that the frontend
// f claims, loaded under ctx as [AssertDocumented] loads them. The surface
// has a line for each public declaration at file level, and a line for
// each relation, each public member and each public nested type of such
// a declaration. The lines of one package follow a line
// "package <path>", in byte order, and the packages follow in the order
// of their paths. A method at file level renders as a member of its host,
// so a Go method and a method in the body of a class render alike. A
// method whose host the graph contains and whose host is not public is
// not in the surface.
//
// A line starts with the modifiers of its declaration, each one word and
// a space: static, abstract, final, sealed, const, async, readonly, get,
// set, delete, constructor and indexer. A line of a declaration at file
// level continues with the spelling of its kind and its name. Every type,
// value and default in a line is the spelling that the frontend recorded.
// A parameter renders as its type, without its name, unless a caller
// passes it by keyword alone. The surface has no comment, no body, and no
// order of declarations or members.
//
// Error modes: Surface returns the error of a source of g that does not
// read, the error of the load, and an error that joins one error for each
// finding of Error severity of the load.
func Surface(ctx context.Context, f plugin.Frontend, g Generated) (string, error) {
	decls, err := loadOutput(ctx, f, g)
	if err != nil {
		return "", err
	}
	byPkg := map[string][]string{}
	for _, d := range decls {
		byPkg[d.pkg] = appendDecl(byPkg[d.pkg], "", d.decl)
	}
	var b strings.Builder
	for _, pkg := range slices.Sorted(maps.Keys(byPkg)) {
		b.WriteString(packageLine + pkg + "\n")
		lines := byPkg[pkg]
		slices.Sort(lines)
		for _, line := range slices.Compact(lines) {
			b.WriteString(line + "\n")
		}
	}
	return b.String(), nil
}

// AssertSurface matches the surface of the files of g that f claims
// against the golden file at path, a slash-separated path relative to the
// directory of the test, through [golden.MatchAt]. Under -update the match
// writes the file. It loads the files under the context of tb, as
// [Surface] loads them. It stops the test where f is nil, where the load
// fails or reports an Error, where the surface has no public declaration,
// and where the golden file differs from the surface.
func AssertSurface(tb assert.TB, f plugin.Frontend, g Generated, path string) {
	tb.Helper()

	assert.NotNil(tb, f, frontendContract)
	lang := string(f.Lang())
	surface, err := Surface(assert.Context(tb), f, g)
	assert.NoError(tb, err, lang+loadsContract)
	assert.NotEmpty(tb, surface, lang+publicContract)
	golden.MatchAt(tb, filepath.FromSlash(path), []byte(surface), golden.ShouldUpdate())
}

// appendDecl appends the lines of one public declaration to lines and
// returns them. host is the name of the declaration that contains a
// nested type, and empty at file level. A host has a line of its own, a
// line for each relation, and the lines of its members and of its public
// nested types. A variant of an enum or a sum is a member without a
// visibility of its own. A method at file level renders as a member of
// the owner of its identity, or of the spelling of its receiver where
// the identity has no owner. A field at file level has no line.
func appendDecl(lines []string, host string, s symbol.Symbol) []string {
	switch d := s.(type) {
	case *node.Struct:
		name := qualified(host, d.Name)
		mods := modifiers([modCount]bool{modAbstract: d.Abstract, modFinal: d.Final, modSealed: d.Sealed})
		lines = append(lines, mods+d.Kind().String()+" "+name+typeParams(d.TypeParams))
		lines = appendRelations(lines, name+relExtends, d.Extends)
		lines = appendRelations(lines, name+relImplements, d.Implements)
		lines = appendRelations(lines, name+relPermits, d.Permits)
		lines = appendEmbeds(lines, name, d.Embeds)
		lines = appendFields(lines, name, d.Fields)
		lines = appendMethods(lines, name, d.Methods)
		return appendNested(lines, name, d.Types)
	case *node.Interface:
		name := qualified(host, d.Name)
		lines = append(lines, modifiers([modCount]bool{modSealed: d.Sealed})+
			d.Kind().String()+" "+name+typeParams(d.TypeParams))
		lines = appendRelations(lines, name+relExtends, d.Extends)
		lines = appendRelations(lines, name+relPermits, d.Permits)
		lines = appendEmbeds(lines, name, d.Embeds)
		lines = appendFields(lines, name, d.Fields)
		lines = appendMethods(lines, name, d.Methods)
		return appendNested(lines, name, d.Types)
	case *node.Enum:
		name := qualified(host, d.Name)
		lines = append(lines, modifiers([modCount]bool{modConst: d.Const})+d.Kind().String()+" "+name)
		lines = appendRelations(lines, name+relImplements, d.Implements)
		for _, v := range d.Variants {
			lines = append(lines, name+"."+v.Name+assigned(v.Value))
		}
		lines = appendFields(lines, name, d.Fields)
		return appendMethods(lines, name, d.Methods)
	case *node.Sum:
		name := qualified(host, d.Name)
		lines = append(lines, d.Kind().String()+" "+name+typeParams(d.TypeParams))
		lines = appendRelations(lines, name+relImplements, d.Implements)
		for _, v := range d.Variants {
			lines = append(lines, name+"."+v.Name+"("+fieldList(v.Fields)+")")
		}
		return appendMethods(lines, name, d.Methods)
	case *node.Alias:
		target := valueMark + spelling(d.Target)
		if d.Defined {
			target = " " + spelling(d.Target)
		}
		return append(lines, d.Kind().String()+" "+qualified(host, d.Name)+typeParams(d.TypeParams)+target)
	case *node.Function:
		return append(lines, modifiers([modCount]bool{modAsync: d.Async})+
			d.Kind().String()+" "+qualified(host, d.Name)+typeParams(d.TypeParams)+params(d.Params)+results(d.Returns))
	case *node.Method:
		return appendMethods(lines, cmp.Or(d.ID.Owner, spelling(d.Receives)), []*node.Method{d})
	case *node.Constant:
		return append(lines, d.Kind().String()+" "+qualified(host, d.Name)+typed(d.Type)+assigned(d.Value))
	case *node.Variable:
		return append(lines, modifiers([modCount]bool{modReadonly: d.Mutability == symbol.MutabilityImmutable})+
			d.Kind().String()+" "+qualified(host, d.Name)+typed(d.Type))
	}
	return lines
}

// appendRelations appends one line for each reference of refs to lines:
// the start of the line, which names the host and the relation, then the
// spelling of the reference.
func appendRelations(lines []string, start string, refs []*node.TypeRef) []string {
	for _, ref := range refs {
		lines = append(lines, start+spelling(ref))
	}
	return lines
}

// appendEmbeds appends one line for each embed of the host to lines.
func appendEmbeds(lines []string, host string, embeds []*node.Embed) []string {
	for _, e := range embeds {
		lines = append(lines, host+relEmbeds+spelling(e.Ref))
	}
	return lines
}

// appendFields appends one line for each public field of the host to
// lines, with the modifiers static and readonly, and the mark of an
// optional field after its name.
func appendFields(lines []string, host string, fields []*node.Field) []string {
	for _, f := range fields {
		if f.Visibility != symbol.VisibilityPublic {
			continue
		}
		optional := ""
		if f.Optional {
			optional = optionalMark
		}
		lines = append(lines, modifiers([modCount]bool{
			modStatic:   f.Level == symbol.LevelType,
			modReadonly: f.Mutability == symbol.MutabilityImmutable,
		})+host+"."+f.Name+optional+typed(f.Type))
	}
	return lines
}

// appendMethods appends one line for each public method of the host to
// lines, with its modifiers, its type parameters, its parameters and its
// results.
func appendMethods(lines []string, host string, methods []*node.Method) []string {
	for _, m := range methods {
		if m.Visibility != symbol.VisibilityPublic {
			continue
		}
		lines = append(lines, modifiers([modCount]bool{
			modStatic:      m.Level == symbol.LevelType,
			modAbstract:    m.Abstract,
			modFinal:       m.Final,
			modAsync:       m.Async,
			modGet:         m.Accessor == symbol.AccessorGet,
			modSet:         m.Accessor == symbol.AccessorSet,
			modDelete:      m.Accessor == symbol.AccessorDelete,
			modConstructor: m.Constructs,
			modIndexer:     m.Indexer,
		})+host+"."+m.Name+typeParams(m.TypeParams)+params(m.Params)+results(m.Returns))
	}
	return lines
}

// appendNested appends the lines of each public nested type of the host
// to lines, with the name of the host and a dot before its own name.
func appendNested(lines []string, host string, types node.Symbols) []string {
	for _, t := range types {
		if visibility(t) == symbol.VisibilityPublic {
			lines = appendDecl(lines, host, t)
		}
	}
	return lines
}

// qualified returns name, with host and a dot before it where host is not
// empty.
func qualified(host, name string) string {
	if host == "" {
		return name
	}
	return host + "." + name
}

// modifiers returns the word of each modifier that on sets, each followed
// by a space, in the order of [modifierWords].
func modifiers(on [modCount]bool) string {
	var b strings.Builder
	for i, set := range on {
		if set {
			b.WriteString(modifierWords[i])
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// typeParams returns the type parameters in brackets, each with its
// bounds after a space and joined by " & ", and the empty string for no
// type parameter.
func typeParams(tps []*node.TypeParam) string {
	if len(tps) == 0 {
		return ""
	}
	parts := make([]string, len(tps))
	for i, tp := range tps {
		bounds := make([]string, len(tp.Bounds))
		for j, bound := range tp.Bounds {
			bounds[j] = spelling(bound)
		}
		parts[i] = tp.Name
		if len(bounds) > 0 {
			parts[i] += " " + strings.Join(bounds, " & ")
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// params returns the parameters in parentheses. Each parameter renders as
// its label and a colon in a language with labels, the variadic mark,
// its type, the mark of an optional parameter, and its default after an
// equals sign. A parameter renders without its name, except a parameter
// that a caller passes by keyword alone: the call names it, so its name
// renders as its label. A parameter that a caller passes by position
// alone renders the positional mark after its type.
func params(ps []*node.Param) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		var b strings.Builder
		switch {
		case p.Label != "":
			b.WriteString(p.Label + labelMark)
		case p.Passing == symbol.PassingKeyword:
			b.WriteString(p.Name + labelMark)
		}
		if p.Variadic != symbol.VariadicNone {
			b.WriteString(variadicMark)
		}
		b.WriteString(spelling(p.Type))
		if p.Passing == symbol.PassingPositional {
			b.WriteString(positionalMark)
		}
		if p.Optional {
			b.WriteString(optionalMark)
		}
		b.WriteString(assigned(p.Default))
		parts[i] = b.String()
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// results returns the types of the results after a space, one type alone
// and a list of more than one in parentheses, and the empty string for no
// result or for one result without a type. A result renders without its
// name.
func results(rs []*node.Return) string {
	switch len(rs) {
	case 0:
		return ""
	case 1:
		return typed(rs[0].Type)
	default:
		parts := make([]string, len(rs))
		for i, r := range rs {
			parts[i] = spelling(r.Type)
		}
		return " (" + strings.Join(parts, ", ") + ")"
	}
}

// fieldList returns the fields of a variant of a sum, each as its name
// and its type, joined by commas.
func fieldList(fields []*node.Field) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.Name + typed(f.Type)
	}
	return strings.Join(parts, ", ")
}

// typed returns the spelling of a type after a space, and the empty
// string for a type that the frontend did not record.
func typed(t *node.TypeRef) string {
	if s := spelling(t); s != "" {
		return " " + s
	}
	return ""
}

// assigned returns a value after an equals sign, and the empty string for
// an empty value.
func assigned(value string) string {
	if value == "" {
		return ""
	}
	return valueMark + value
}
