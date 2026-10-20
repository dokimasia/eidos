// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"
	"text/template"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// The names of the vocabulary are constants, so a template and the map of
// its helpers use the same name. The names that every backend declares are
// the names of the render package.
const (
	// FuncDocs is the template name of [Docstring] at the level of a module.
	FuncDocs = render.HelperDocs
	// FuncSpell is the template name of [Writer.Spell].
	FuncSpell = render.HelperSpell
	// FuncSpellValue is the template name of [Writer.SpellValue].
	FuncSpellValue = render.HelperSpellValue
	// FuncTypeParams is the template name of [Writer.TypeParams].
	FuncTypeParams = render.HelperTypeParams
	// FuncTypeArgs is the template name of the type arguments of a list of
	// type parameters or of type references, in brackets.
	FuncTypeArgs = render.HelperTypeArgs
	// FuncParams is the template name of the parameter group of a slot of
	// parameters.
	FuncParams = render.HelperParams
	// FuncResults is the template name of the result annotation of a slot of
	// results.
	FuncResults = render.HelperResults
	// FuncWrapDocs is the template name of [WrapDocs].
	FuncWrapDocs = render.HelperWrapDocs
	// FuncDecorators is the template name of [Writer.Decorators].
	FuncDecorators = "decorators"
	// FuncStruct is the template name of [Writer.Struct].
	FuncStruct = "struct"
	// FuncInterface is the template name of [Writer.Interface].
	FuncInterface = "interface"
	// FuncEnum is the template name of [Writer.Enum].
	FuncEnum = "enum"
	// FuncMethod is the template name of [Writer.Method].
	FuncMethod = "method"
	// FuncFunction is the template name of [Writer.Function].
	FuncFunction = "function"
	// FuncAlias is the template name of [Writer.Alias].
	FuncAlias = "alias"
	// FuncConstant is the template name of [Writer.Constant].
	FuncConstant = "constant"
	// FuncVariable is the template name of [Writer.Variable].
	FuncVariable = "variable"
	// FuncModule is the template name of a module with a header and its
	// declarations: its head, as [Writer.Head] writes it, its import block,
	// whose lazy marker names the modules that the block imports lazily, its
	// __all__, as [Writer.All] writes it, its type variables, as
	// [Writer.TypeVars] writes them, and the declarations, with a blank line
	// after each part. The import block leaves out each import whose bound
	// name is no run of the characters of an identifier in the parts after
	// it, which the helper records in the import set. A re-export of the
	// header, as from a import b as b, stays, because a type checker reads
	// it as a public name of the module.
	FuncModule = "module"
)

// vocabulary returns the template vocabulary of a module, or of a stub file
// where stub is true, with the standard library of im for the import block.
// Each call of the returned function binds a writer of its own to set, the
// import set of one file, and each helper records the imports of what it
// writes into set. The start of every module returns the error of a table
// of the standard library that does not parse. A call allocates the writer,
// the helpers that are no methods of the writer, and the map of the
// helpers.
func vocabulary(im importer, stub bool) func(set *render.ImportSet) template.FuncMap {
	return func(set *render.ImportSet) template.FuncMap {
		w := NewWriter(set, stub)
		return template.FuncMap{
			FuncDocs:       func(lines []string) string { return Docstring(lines, "") },
			FuncSpell:      w.Spell,
			FuncSpellValue: w.SpellValue,
			FuncTypeParams: w.TypeParams,
			FuncTypeArgs:   w.typeArgs,
			FuncParams:     w.paramSlot,
			FuncResults:    w.resultSlot,
			FuncWrapDocs:   WrapDocs,
			FuncDecorators: w.Decorators,
			FuncStruct:     w.Struct,
			FuncInterface:  w.Interface,
			FuncEnum:       w.Enum,
			FuncMethod:     w.Method,
			FuncFunction:   w.Function,
			FuncAlias:      w.Alias,
			FuncConstant:   w.Constant,
			FuncVariable:   w.Variable,
			// The head binds the imports of the header, and the parts after the
			// import block write their imports, before the block reads the set.
			FuncModule: func(h *emit.File, decls string) (string, error) {
				head, headErr := w.Head(h)
				if err := errors.Join(im.err, headErr); err != nil {
					return "", err
				}
				rest := w.All(h) + newline + w.TypeVars() + newline + decls
				for _, e := range set.Entries() {
					bound, prunable := boundOf(e)
					if prunable && !set.Has(reexportPath, bound) && !identifiers(rest, bound) {
						set.AddNamed(unusedPath, bound)
					}
				}
				return head + newline + im.Imports(set) + newline + rest, nil
			},
		}
	}
}

// WrapDocs wraps text at [textfmt.DocsWidth] into the lines that
// [Docstring] writes. griffe reads a docstring as Markdown, so the wrap
// starts no line with a block opener of CommonMark. A run of dashes or of
// equals signs is among them, which is also the underline of a section of
// reStructuredText. It allocates what [textfmt.Wrap] allocates.
func WrapDocs(text string) []string {
	return textfmt.Wrap(text, textfmt.DocsWidth, textfmt.MarkupMarkdown)
}

// typeArgs writes a list of type parameters or of type references as the
// arguments of a reference in brackets, as [T] or [int, str], and nothing
// for an empty list.
func (w *Writer) typeArgs(list any) (string, error) {
	return spellref.TypeArgs(list, argsOpen, argsClose, w.Spell)
}

// paramSlot writes the parameter group of the parameters of a slot, as
// [Writer.Method] writes the parameters of a static method.
func (w *Writer) paramSlot(ps *emit.Slot[*emit.Param]) (string, error) {
	return w.params(callable{params: ps.Items()})
}

// resultSlot writes the result annotation of the results of a slot, as
// [Writer.Method] writes it.
func (w *Writer) resultSlot(rs *emit.Slot[*emit.Return]) (string, error) {
	return w.results(rs.Items())
}
