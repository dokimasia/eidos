// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/python"
	pygrammar "go.dokimi.dev/eidos/lang/treesitter/python"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration. It restates the
// constant of the satellite root for the callers of the frontend.
const Lang = python.Lang

// UnparsedFile reports a syntax error at its position. The error is a
// problem of the source, and the file contributes every declaration
// outside the statements with an error.
var UnparsedFile = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a Python file has a syntax error",
})

// BadCarrier reports a directive carrier that the kernel grammar refused.
// The carrier attaches nothing, and the load continues.
var BadCarrier = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier or a decorator of the brand
// on a subject that the model cannot address, such as an import, a
// statement or a comment that no declaration takes. The author learns that
// the directive attached nowhere.
var UnaddressedCarrier = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// UnreadProject reports a pyproject.toml that does not read or decode, at
// its first line. Severity is Warning. The files of the project take the
// root of the run of package directories that contain them.
var UnreadProject = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a pyproject.toml does not read or parse",
})

// UnmodeledConstruct reports a construct without a form in the model, at
// the construct: a construct of Python 2 in a signature that the grammar
// keeps, such as a tuple parameter, and a class that an enum declares.
// Severity is Warning. The declaration loads without the construct, and the
// carriers and the decorators of the brand on such a class report under
// [UnaddressedCarrier].
var UnmodeledConstruct = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  5,
	Meaning: "a construct of the source has no form in the model",
})

// DuplicateName reports a second declaration of a name in a module or in a
// class, at the second declaration. Severity is Info. The first declaration
// is the declaration of the name, and the overloads and the accessors of one
// name are one declaration.
var DuplicateName = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "a module or a class declares a name a second time",
})

// ForeignEncoding reports an encoding declaration that names an encoding
// other than UTF-8, at the declaration. Severity is Warning. The file parses
// as UTF-8, and its bytes keep their values.
var ForeignEncoding = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  7,
	Meaning: "the encoding declaration of a file names an encoding other than UTF-8",
})

// BadMarker reports a decorator of the brand that is not a valid directive.
// Either its path is not the brand followed by a name, or by a plugin and a
// name, or an argument is not a literal that the frontend lifts. The
// decorator attaches nothing and remains an annotation.
var BadMarker = diag.MustRegister(python.CodePrefix, diag.CodeSpec{
	Number:  8,
	Meaning: "a decorator of the brand is not a valid directive",
})

// The values of the options that the empty options take.
const (
	defaultVersion  = python.Version315
	defaultPlatform = "linux"
)

// selection is the claim of files: the modules and the stub files of the
// workspace, without the virtual environments, the installed packages, the
// caches of the interpreter and of tox and nox, and the packages of npm.
var selection = []string{
	"**/*" + python.Extension, "**/*" + python.StubExtension,
	"!**/.venv/**", "!**/venv/**", "!**/site-packages/**", "!**/__pycache__/**", "!**/.tox/**",
	"!**/.nox/**", "!**/node_modules/**",
}

// Options is the declared configuration of the frontend, which every unit
// key folds.
type Options struct {
	// Version is the version of Python that the checks of sys.version_info
	// compare with, and the version that typeshed's VERSIONS selects the
	// standard library for, as 3.15. The empty version is 3.15.
	Version string

	// Platform is the value of sys.platform that the checks compare with,
	// such as linux, darwin or win32. The empty platform is linux.
	Platform string
}

// pyFrontend is the vocabulary of the grammar, which every parse reads and
// none writes, the configuration of the load, and the memo of the decoded
// project files, which the parses share.
type pyFrontend struct {
	v        *vocabulary
	opts     *Options
	projects projectMemo
}

// target returns the minor version of Python 3 and the platform of the
// options, with their defaults for the empty options.
//
// Error modes: target returns an error for a version that is not a version
// of Python 3, as 3.15.
func (f *pyFrontend) target() (minor int, platform string, err error) {
	version := f.opts.Version
	if version == "" {
		version = string(defaultVersion)
	}
	minor, read := python.Minor(version)
	if !read {
		return 0, "", fmt.Errorf("frontend: the option Version %q is not a version of Python 3, such as 3.15", version)
	}
	platform = f.opts.Platform
	if platform == "" {
		platform = defaultPlatform
	}
	return minor, platform, nil
}

// New builds the Python frontend through the kit. Nil options read Python
// 3.15 on linux. The frontend declares that the language overloads, so the
// discriminator of a callable contains the types of its parameters. It
// implements the exporter role, so a reference through a name that a module
// imports resolves to the declaration that the name binds, and the
// dependent role over the stores of [Stores]. Its version folds the version
// of the grammar. The kit stamps python.testFile on every test file of
// pytest, and registers every python key as the key provider of the
// frontend.
//
// # Allocation contract
//
// New allocates the state of the frontend with its vocabulary and its parse
// and dependency hooks, the two functions of the comment walk that capture
// the vocabulary, the version, the classifier of test files, the syntax and
// five allocations of the kit: 14 allocations, and 15 with the empty
// options that nil options take.
func New(opts *Options) plugin.Frontend {
	if opts == nil {
		opts = &Options{}
	}
	f := &pyFrontend{v: newVocabulary(pygrammar.Grammar), opts: opts}
	return frontend.New(python.Name, Lang, python.Syntax()).
		Version(python.FrontendVersion + "; " + pygrammar.Grammar.Version()).
		Overloads().
		Options(opts).
		Match(selection...).
		Units(partition).
		Parse(f.parse).
		Classify(frontend.TestFiles(python.TestFileKey, testFile)).
		Resolve(resolve).
		Exports(exports).
		Dependencies(f.dependencies).
		Stores(Stores).
		Keys(python.Keys).
		Build()
}
