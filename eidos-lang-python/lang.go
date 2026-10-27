// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language in the [symbol.Identity] of every
// declaration that the frontend loads. A composition registers the
// projection rules of the satellite under it.
const Lang symbol.Lang = "python"

// CodePrefix opens every diagnostic code that the satellite registers, such
// as PYTHON-0001.
const CodePrefix diag.Prefix = "PYTHON"

// Name is the one identity of the satellite. Its findings report under the
// name, and the kernel attributes the stamps of its frontend to the name. A
// composition claims the python namespace of metadata for the name.
const Name plugin.ID = "python"

// The extensions of the files that the frontend claims: a module of
// source, and a stub file, which states the types of a module without its
// implementation.
const (
	Extension     = ".py"
	StubExtension = ".pyi"
)

// InitModule is the base name of the module of a package directory, as in
// __init__.py. The module of shop/__init__.py is the package shop.
const InitModule = "__init__"

// PackageSep separates the segments of the package of a module, as in
// shop/models for the module shop.models. A package path is
// slash-separated, as every package path of the kernel is.
const PackageSep = "/"

// ModuleSep separates the segments of the dotted name of a module, as in
// shop.models.
const ModuleSep = "."

// Indent is one level of indentation, four spaces, as the formatter of ruff
// writes it.
const Indent = "    "

// FrontendVersion is the version of what the frontend writes into the
// graph, and not the release of the module. Every unit key folds it beside
// the version of the grammar. A change of the load changes it, which
// invalidates every unit that the old value keyed.
const FrontendVersion = "0.2.0"

// Target is the rendering target through which a plan selects the Python
// backend. Its spelling is the spelling of [Lang], so the emitter copies a
// reference of a Python source as written into a plan of the target.
const Target plugin.Target = "python"

// BackendVersion is the version of the output of the backend. The
// composition fingerprint folds it, and every change of the rendered output
// changes it.
const BackendVersion = "0.3.0"

// Syntax returns the comment forms of Python: the line comment # and no
// block comment. A docstring is a string, and the frontend reads it as
// documentation apart from the comments. A comment of Python states a tool
// directive as prose, such as # type: ignore, so the syntax does not
// declare the directive convention. A call returns a fresh value, which the
// caller may keep. It allocates the list of line openers.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"#"}}
}
