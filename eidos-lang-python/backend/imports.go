// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
	"go.dokimi.dev/eidos/sdk/render"
)

// The marks of the paths of the entries of an import set. An entry whose
// path ends in moduleMark imports the module under the name of the entry, as
// in import numpy as np. An entry whose path starts with specialMark is no
// import: a type variable, whose path starts with typeVarMark and continues
// with the call that declares the variable, a public name of the file, a
// module that the file imports lazily, and a function of the file that no
// overload decorator marks. No module path contains the marks.
const (
	moduleMark  = "\x00"
	specialMark = "\x00"
	typeVarMark = specialMark + "typevar" + specialMark
)

// The modules of the import block that the sections place apart: the
// module of the future statement, which has a section of its own, and the
// modules that typeshed lists beside the standard library, which isort
// counts among the third-party modules.
const (
	futureModule           = "__future__"
	typingExtensionsModule = "typing_extensions"
	typeshedModule         = "_typeshed"
)

// The words of an import statement and the parts of a wrapped one.
const (
	lazyWord     = "lazy "
	space        = " "
	importOpen   = " (\n"
	importClose  = ")\n"
	statementEnd = "\n"
	sectionSep   = "\n"
)

// section is the section of an import block that a module belongs to, in
// the order of the block.
type section uint8

// The sections of an import block, in the order of isort's defaults: the
// future statement, the standard library, the third-party modules and the
// modules of the project.
const (
	futureSection section = iota
	stdlibSection
	thirdPartySection
	projectSection
)

// memberType is the kind of name that a member of an import statement has,
// in the order in which isort's order by type sorts the members: a
// constant in capitals, a class in capitalized words, and every other name.
type memberType uint8

const (
	constantMember memberType = iota
	classMember
	otherMember
)

// importer renders the import block of a file. It reads the standard
// library of each version from the VERSIONS table of typeshed, and err is
// the error of a table that does not parse, which the start of every module
// returns.
//
// # Concurrency
//
// An importer is safe for concurrent use, because it never writes its table.
type importer struct {
	versions typeshed.Versions
	err      error
}

// statement is one import statement of the block: import a.b, import a.b as
// c, or from a.b import with its members. module is the dotted name of the
// module, and section the section of the block that the statement is in.
type statement struct {
	module  string
	alias   string
	members []member
	section section
	from    bool
	lazy    bool
}

// member is one name of a from statement, with the name that it binds
// under where the statement renames it.
type member struct {
	name  string
	alias string
}

// Imports renders the import block of the file whose import set is set, in
// the layout of isort's defaults as ruff applies them:
//
//   - the sections in order: the future statement, the standard library of
//     the target version, the third-party modules and the modules of the
//     project, with a blank line between two sections. A module of the
//     project is a module of the top-level package of the file or a module
//     of a file of the plan that the file refers into. typing_extensions
//     and _typeshed are third-party modules, as isort counts them;
//   - in a section, the statements import a.b first and then the
//     statements from a.b import, each sorted by module without regard to
//     case. Two statements of one module sort by their first member;
//   - the members of a statement without renaming are one statement,
//     sorted as isort's order by type sorts them: constants, then classes,
//     then the other names, each without regard to case. A renamed member,
//     a re-export of the header with its redundant alias, as
//     from a import b as b, and a star import are statements of their own;
//   - a from statement longer than the line puts one member on each line,
//     in parentheses with a trailing comma;
//   - a statement of a module that the file imports lazily starts with the
//     word lazy. [Writer.Head] records the modules of the lazy marker of the
//     header in the set;
//   - an import whose bound name the set records as unused stays out, as
//     the start of a module records the names that the rest of the module
//     does not use.
//
// Imports leaves out every entry that is no import, and returns the empty
// string for a set without imports. It is the import renderer of the
// backend kit, and the import block of the start of a module.
//
// # Allocation contract
//
// Imports allocates the sorted entries of the set, the statements, the
// members of the statements, the dotted name of each module of a package,
// and the block, which it sizes once. A set without imports allocates its
// entries alone.
func (im importer) Imports(set *render.ImportSet) string {
	entries := set.Entries()
	imports := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Path, specialMark) {
			imports++
		}
	}
	if imports == 0 {
		return ""
	}
	// Each import adds one statement or one member at most, so neither list
	// grows.
	stmts := make([]statement, 0, imports)
	members := make([]member, 0, imports)
	for rest := entries; len(rest) > 0; {
		n := 1
		for n < len(rest) && rest[n].Path == rest[0].Path {
			n++
		}
		run := rest[:n]
		rest = rest[n:]
		path := run[0].Path
		if strings.HasPrefix(path, specialMark) {
			continue
		}
		path, aliased := strings.CutSuffix(path, moduleMark)
		st := statement{
			module: strings.ReplaceAll(path, python.PackageSep, python.ModuleSep), section: im.section(set, path),
		}
		st.lazy = slices.Contains(entries, render.Entry{Path: lazyPath, Name: st.module})
		for _, e := range run {
			if bound, prunable := boundOf(e); prunable && set.Has(unusedPath, bound) {
				continue
			}
			one := st
			switch {
			case aliased:
				one.alias = e.Name
			case e.Name == "":
			case e.Name == python.StarName:
				members = append(members, member{name: e.Name})
				one.from, one.members = true, members[len(members)-1:len(members):len(members)]
			case e.Item != "":
				members = append(members, member{name: e.Item, alias: e.Name})
				one.from, one.members = true, members[len(members)-1:len(members):len(members)]
			case set.Has(reexportPath, e.Name):
				members = append(members, member{name: e.Name, alias: e.Name})
				one.from, one.members = true, members[len(members)-1:len(members):len(members)]
			default:
				continue
			}
			stmts = append(stmts, one)
		}
		start := len(members)
		for _, e := range run {
			plain := !aliased && e.Name != "" && e.Name != python.StarName && e.Item == ""
			if plain && !set.Has(unusedPath, e.Name) && !set.Has(reexportPath, e.Name) {
				members = append(members, member{name: e.Name})
			}
		}
		if names := members[start:len(members):len(members)]; len(names) > 0 {
			slices.SortFunc(names, compareMembers)
			st.from, st.members = true, names
			stmts = append(stmts, st)
		}
	}
	slices.SortFunc(stmts, func(a, b statement) int {
		return cmp.Or(cmp.Compare(a.section, b.section), compareStatements(a, b))
	})
	size := 0
	for i, st := range stmts {
		if i > 0 && st.section != stmts[i-1].section {
			size += len(sectionSep)
		}
		size += st.size()
	}
	var b strings.Builder
	b.Grow(size)
	for i, st := range stmts {
		if i > 0 && st.section != stmts[i-1].section {
			b.WriteString(sectionSep)
		}
		st.write(&b)
	}
	return b.String()
}

// boundOf returns the name that the import of the entry e binds, and
// reports whether the block can leave the import out: an import that is no
// future statement and no star import, because the first changes how the
// module compiles and the second binds names that the set does not list.
// The bound name of import a.b is a, and the bound name of every other
// import is the name that it binds. An entry that is no import reports
// false.
func boundOf(e render.Entry) (string, bool) {
	path, aliased := strings.CutSuffix(e.Path, moduleMark)
	switch {
	case strings.HasPrefix(e.Path, specialMark), path == futureModule, e.Name == python.StarName:
		return "", false
	case e.Name == "" && !aliased:
		top, _, _ := strings.Cut(path, python.PackageSep)
		return top, true
	default:
		return e.Name, true
	}
}

// identifiers reports whether name is one of the runs of the characters of
// an identifier in text: letters, digits and underscores. A name inside a
// string is such a run, so a name of __all__ and of a string annotation
// counts. identifiers allocates nothing.
func identifiers(text, name string) bool {
	start := -1
	for i, r := range text {
		identifier := r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
		switch {
		case identifier && start < 0:
			start = i
		case !identifier && start >= 0:
			if text[start:i] == name {
				return true
			}
			start = -1
		}
	}
	return start >= 0 && text[start:] == name
}

// section returns the section of the module whose package path is path, in
// the file whose import set is set.
func (im importer) section(set *render.ImportSet, path string) section {
	top, _, _ := strings.Cut(path, python.PackageSep)
	home, _, _ := strings.Cut(set.Home(), python.PackageSep)
	_, referent := set.Referent(path)
	switch {
	case path == futureModule:
		return futureSection
	case top == home, referent:
		return projectSection
	case top == typingExtensionsModule, top == typeshedModule:
		return thirdPartySection
	case im.versions.Includes(path, minorOf(set)):
		return stdlibSection
	default:
		return thirdPartySection
	}
}

// write writes the statement into b: on one line where it fits, and a from
// statement with one member on each line otherwise. It writes the parts of
// the statement into b alone, so it allocates nothing beyond the growth of
// b.
func (st statement) write(b *strings.Builder) {
	if st.lazy {
		b.WriteString(lazyWord)
	}
	if !st.from {
		b.WriteString(python.ImportWord)
		b.WriteString(space)
		b.WriteString(st.module)
		if st.alias != "" {
			b.WriteString(space)
			b.WriteString(python.AsWord)
			b.WriteString(space)
			b.WriteString(st.alias)
		}
		b.WriteString(statementEnd)
		return
	}
	b.WriteString(python.FromWord)
	b.WriteString(space)
	b.WriteString(st.module)
	b.WriteString(space)
	b.WriteString(python.ImportWord)
	if st.width() <= lineWidth {
		b.WriteString(space)
		for i, m := range st.members {
			if i > 0 {
				b.WriteString(itemsSep)
			}
			m.write(b)
		}
		b.WriteString(statementEnd)
		return
	}
	b.WriteString(importOpen)
	for _, m := range st.members {
		b.WriteString(python.Indent)
		m.write(b)
		b.WriteString(itemEnd + statementEnd)
	}
	b.WriteString(importClose)
}

// width returns the length in characters of a from statement on one line,
// which [statement.write] compares with the width of a line.
func (st statement) width() int {
	n := len(python.FromWord) + 3*len(space) + utf8.RuneCountInString(st.module) + len(python.ImportWord)
	if st.lazy {
		n += len(lazyWord)
	}
	for i, m := range st.members {
		if i > 0 {
			n += len(itemsSep)
		}
		n += utf8.RuneCountInString(m.name)
		if m.alias != "" {
			n += 2*len(space) + len(python.AsWord) + utf8.RuneCountInString(m.alias)
		}
	}
	return n
}

// size returns the length in bytes of the text that [statement.write]
// writes at most: the longer of the statement on one line and the statement
// with one member on each line.
func (st statement) size() int {
	n := len(lazyWord) + len(python.FromWord) + len(python.ImportWord) + 3*len(space) + len(st.module) +
		len(python.AsWord) + len(st.alias) + len(statementEnd) + len(importOpen) + len(importClose)
	for _, m := range st.members {
		n += len(itemsSep) + len(python.Indent) + len(m.name) + 2*len(space) + len(python.AsWord) + len(m.alias) +
			len(itemEnd) + len(statementEnd)
	}
	return n
}

// write writes the member into b as a from statement writes it: the name,
// and the name with its alias after the word as where the member is
// renamed.
func (m member) write(b *strings.Builder) {
	b.WriteString(m.name)
	if m.alias != "" {
		b.WriteString(space)
		b.WriteString(python.AsWord)
		b.WriteString(space)
		b.WriteString(m.alias)
	}
}

// compareStatements orders two statements of one section: an import
// statement before a from statement, then the modules without regard to
// case and then with it, then the first members.
func compareStatements(a, b statement) int {
	switch {
	case a.from != b.from && !a.from:
		return -1
	case a.from != b.from:
		return 1
	}
	if c := cmp.Or(compareNatural(a.module, b.module), strings.Compare(a.module, b.module)); c != 0 {
		return c
	}
	if a.from {
		return compareMembers(a.members[0], b.members[0])
	}
	return strings.Compare(a.alias, b.alias)
}

// compareMembers orders two members of a from statement as isort's order by
// type orders them: by the type of the name, then by the name without regard
// to case and then with it, then by the alias.
func compareMembers(a, b member) int {
	return cmp.Or(
		cmp.Compare(typeOf(a.name), typeOf(b.name)),
		compareNatural(a.name, b.name),
		strings.Compare(a.name, b.name),
		strings.Compare(a.alias, b.alias),
	)
}

// typeOf returns the type of a member name: a constant for a name of more
// than one character that has an upper-case letter and no lower-case one, a
// class for a name that starts with an upper-case letter, and every other
// name otherwise. It allocates nothing.
func typeOf(name string) memberType {
	upper, lower := false, false
	for _, r := range name {
		upper = upper || unicode.IsUpper(r)
		lower = lower || unicode.IsLower(r)
	}
	first, _ := utf8.DecodeRuneInString(name)
	switch {
	case len(name) > 1 && upper && !lower:
		return constantMember
	case unicode.IsUpper(first):
		return classMember
	default:
		return otherMember
	}
}

// compareNatural orders two texts naturally and without regard to case, as
// isort orders modules and members: a run of digits compares as its
// number, and every other character as its lower case. It allocates
// nothing.
func compareNatural(a, b string) int {
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if c := cmp.Or(cmp.Compare(len(na), len(nb)), strings.Compare(na, nb)); c != 0 {
				return c
			}
			a, b = a[da:], b[db:]
			continue
		}
		ra, sa := utf8.DecodeRuneInString(a)
		rb, sb := utf8.DecodeRuneInString(b)
		if c := cmp.Compare(unicode.ToLower(ra), unicode.ToLower(rb)); c != 0 {
			return c
		}
		a, b = a[sa:], b[sb:]
	}
	return cmp.Compare(len(a), len(b))
}

// digits returns the length of the run of ASCII digits at the start of s.
func digits(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}

// TemplateImport returns the binding of an import that a template states
// through the use builtin: use "dataclasses" "field" binds field from the
// module dataclasses, as from dataclasses import field. An import without a
// name imports the whole module, as use "json" writes import json, and does
// not bind a name in the name assignment of the file, so TemplateImport
// reports false for it. TemplateImport does not allocate.
func TemplateImport(_, name string) (local, item string, ok bool) {
	if name == "" {
		return "", "", false
	}
	return name, name, true
}
