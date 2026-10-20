// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix is the start of every refusal of the backend. It is the
// identity of the language, as in the refusals of every backend.
const refusalPrefix = string(python.Lang) + ": "

// The minor versions of Python 3 at which the written syntax changes:
// PEP 695 type parameters and the type statement at 3.12, the defaults of
// PEP 696 and the typing names of 3.13 at 3.13, the deferred annotations
// of PEP 649 at 3.14, and lazy imports at 3.15. defaultMinor is the
// version of the default choice of the version policy.
const (
	minor312     = 12
	minor313     = 13
	minor314     = 14
	minor315     = 15
	defaultMinor = 11
)

// errNoStatement is the refusal of an argument of the import marker that is
// no import statement.
var errNoStatement = errors.New(refusalPrefix + "an argument of the import marker is no import statement")

// Writer writes the parts of the declarations of one Python file that the
// templates compose: types, type parameters, signatures, decorators,
// docstrings, class heads, members, and the head of the file. It records
// the import of every declaration of another module that it writes into
// the import set of the file. It writes a module, or a stub file where it
// writes stubs.
//
// The declaration under writing is the declaration of the kind writer that
// runs, and in a body the callable and the host that the import set names
// for the body. The class under writing is the class whose fields a class
// writer writes, or the host of the method whose signature the writer
// writes. Its members hide the builtins of their names.
//
// The target version is the choice of the policy python.version of the
// policy of the set, and 3.11 where the policy does not declare the key.
// The version decides the syntax of type parameters and aliases, the module
// of a name of typing that a later version adds, and whether an annotation
// quotes a declaration of the file that has not rendered yet.
//
// # Concurrency
//
// A Writer is not safe for concurrent use, because its set is not. The
// pass binds one vocabulary to each worker.
//
// # Allocation contract
//
// Each method allocates the text that it writes and the imports that the
// set records. The first method of a file that binds its header parses the
// import marker of the header once, and allocates the statements. The first
// type variable of each call allocates the path of its entry, and a later
// type variable of the same call allocates none.
type Writer struct {
	set  *render.ImportSet
	stub bool
	// header is the header whose import marker imports contains, which the
	// writer parsed for the file under render.
	header  *emit.File
	imports []importLine
	// scope lists the type parameters of the declaration under writing in
	// the order of their declaration, each with the name of the type
	// variable of the module that it takes. A parameter of the syntax of
	// PEP 695 takes its own name.
	scope []rename
	// calls maps each call that declared a type variable to the path of its
	// entry in the import set.
	calls map[varCall]string
	// class is the class under writing while a class writer writes its
	// fields and while a method writes its signature, and nil otherwise.
	class symbol.Symbol
}

// NewWriter returns the writer of the file whose import set is set: the
// writer of a module, or of a stub file where stub is true. set must not be
// nil. NewWriter allocates the writer.
func NewWriter(set *render.ImportSet, stub bool) *Writer {
	return &Writer{set: set, stub: stub}
}

// importLine is one statement of the import marker of a header: the package
// path of its module, the name that it imports, and the name that it binds.
// name is empty for an import of the whole module, and the star for a star
// import. local is empty where the statement binds the name of the module
// or of the item itself.
type importLine struct {
	path  string
	name  string
	local string
}

// minorOf returns the minor version of Python 3 that the policy of the set
// selects: the choice of python.version, and 3.11 for a set whose policy
// does not declare the key or chooses no version of Python 3. It allocates
// nothing.
func minorOf(set *render.ImportSet) int {
	c, declared := set.Policy().Lookup(python.VersionPolicy)
	if minor, valid := python.Minor(string(c)); declared && valid {
		return minor
	}
	return defaultMinor
}

// Bind binds the imports of the import marker of the header h in the import
// set of the file, so a declaration of the file refers to an imported name
// by the name that the import binds. Every kind template binds the header
// before it writes, because a template writes before the skeleton does. A
// statement that the set binds already binds nothing more, so a second call
// changes nothing. Bind returns the empty string.
//
// Error modes: an argument of the marker that is no import statement, and
// a name that another import or a declaration of the file binds.
func (w *Writer) Bind(h *emit.File) (string, error) {
	if h == nil {
		return "", nil
	}
	if h != w.header {
		lines, err := parseImports(h)
		if err != nil {
			return "", err
		}
		w.header, w.imports = h, lines
	}
	for _, l := range w.imports {
		if err := w.bindLine(l); err != nil {
			return "", err
		}
	}
	return "", nil
}

// bindLine binds one statement of the import marker in the import set. A
// statement whose alias repeats the name that it imports, as import json as
// json and from a import b as b, is the explicit re-export of the typing
// specification, which a type checker reads as a public name of the module.
// bindLine records its bound name as a re-export, so the import block keeps
// the alias and the statement.
func (w *Writer) bindLine(l importLine) error {
	local := cmp.Or(l.local, l.name)
	var err error
	switch {
	case l.name == python.StarName:
		w.set.AddNamed(l.path, python.StarName)
		return nil
	case local == "":
		w.set.Add(l.path)
		return nil
	case l.name == "":
		err = w.set.Use(l.path+moduleMark, local, "")
	default:
		err = w.set.Use(typingPath(l.path, l.name, minorOf(w.set)), local, l.name)
	}
	if err != nil {
		return refuse("the header binds %s to an import of %s, and the file binds the name or the import otherwise",
			local, strings.ReplaceAll(l.path, python.PackageSep, python.ModuleSep))
	}
	if l.local != "" && (l.local == l.name || l.name == "" && l.local == l.path) {
		w.set.AddNamed(reexportPath, local)
	}
	return nil
}

// parseImports parses the arguments of the import marker of the header h.
// It returns no statement for a header without the marker.
func parseImports(h *emit.File) ([]importLine, error) {
	var out []importLine
	for _, a := range h.Annotations {
		if a.Name != string(python.ImportMarker) {
			continue
		}
		for _, arg := range a.Args {
			l, err := parseImport(arg)
			if err != nil {
				return nil, err
			}
			out = append(out, l)
		}
	}
	return out, nil
}

// parseImport parses one statement of the import marker: import a.b,
// import a.b as c, from a.b import c, from a.b import c as d, or
// from a.b import *.
//
// The path of the line is the package path of the dotted name of the
// module, as shop/models for shop.models.
func parseImport(stmt string) (importLine, error) {
	words := strings.Fields(stmt)
	if len(words) < 2 {
		return importLine{}, fmt.Errorf("%w: %q", errNoStatement, stmt)
	}
	l := importLine{path: strings.ReplaceAll(words[1], python.ModuleSep, python.PackageSep)}
	switch {
	case len(words) == 2 && words[0] == python.ImportWord:
	case len(words) == 4 && words[0] == python.ImportWord && words[2] == python.AsWord:
		l.local = words[3]
	case len(words) == 4 && words[0] == python.FromWord && words[2] == python.ImportWord:
		l.name = words[3]
	case len(words) == 6 && words[0] == python.FromWord && words[2] == python.ImportWord &&
		words[4] == python.AsWord:
		l.name, l.local = words[3], words[5]
	default:
		return importLine{}, fmt.Errorf("%w: %q", errNoStatement, stmt)
	}
	return l, nil
}

// refuse returns a refusal that starts with [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
