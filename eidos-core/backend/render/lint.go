// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"text/template"
	"text/template/parse"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// Lint checks the template tree of one plugin against the static half of
// the template rules, for the target of the pass. It returns one error for
// each finding, and nothing for a valid tree:
//
//   - A template that does not parse with the merged vocabulary, so an
//     unknown function fails here and not at execute time.
//   - A body template without a slots or slot marker, because the render
//     would strand a pending contribution.
//   - A helper that shadows a shared name without a declared override,
//     which the render refuses too.
//   - A declared override that replaces no shared name.
//
// files are the paths of the file templates of the tree, from the
// [plugin.Output.Template] fields of the plugin. A file template parses
// with the builtins of a file template and needs no marker, because it
// places the body of a record through the body builtin. The lint does
// not see the payload of a reference or the text inside verbatim.
func (p *Pass) Lint(tree fs.FS, funcs template.FuncMap, overrides, files []string) []error {
	var findings []error

	declared := map[string]bool{}
	for _, name := range overrides {
		declared[name] = true
		if _, shared := p.shared[name]; !shared {
			findings = append(findings, fmt.Errorf(
				"render: the override %q replaces nothing in %s's shared vocabulary",
				name, p.name,
			))
		}
	}
	vocabulary := template.FuncMap{}
	maps.Copy(vocabulary, p.shared)
	for _, name := range slices.Sorted(maps.Keys(funcs)) {
		_, shared := p.shared[name]
		switch admit(name, shared, declared[name]) {
		case claimsBuiltin:
			findings = append(findings, fmt.Errorf(
				"render: the helper %q claims a builtin's name", name,
			))
		case shadowsUndeclared:
			findings = append(findings, fmt.Errorf(
				"render: the helper %q shadows %s's shared vocabulary without declaring the override",
				name, p.name,
			))
		case admitted:
			vocabulary[name] = funcs[name]
		}
	}
	// The stubs define exactly the names a plugin tree's execution
	// binds, from the one constructor the render uses, so a builtin
	// the render refuses fails here too.
	stubs := referenceBuiltins(
		func() (string, error) { return "", nil },
		func(string) (string, error) { return "", nil },
		func(string, ...string) (string, error) { return "", nil },
	)
	fileStubs := fileBuiltins(
		func(any) (string, error) { return "", nil },
		func(*emit.Method) (string, error) { return "", nil },
		func(string, ...string) (string, error) { return "", nil },
		func(string, symbol.Symbol) (string, error) { return "", nil },
	)

	walk := func(name string) error {
		src, err := fs.ReadFile(tree, name)
		if err != nil {
			return fmt.Errorf("render: reading %s: %w", name, err)
		}
		if slices.Contains(files, name) {
			if _, perr := template.New(name).Funcs(fileStubs).Funcs(vocabulary).Parse(string(src)); perr != nil {
				return fmt.Errorf("render: %w", perr)
			}
			return nil
		}
		t, err := template.New(name).Funcs(stubs).Funcs(vocabulary).Parse(string(src))
		if err != nil {
			return fmt.Errorf("render: %w", err)
		}
		if !placesSlots(t.Root) {
			return fmt.Errorf(
				"render: %s places no %s or %s marker, and a pending contribution would strand",
				name, BuiltinSlots, BuiltinSlot,
			)
		}
		return nil
	}
	err := fs.WalkDir(tree, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ferr := walk(name); ferr != nil {
			findings = append(findings, ferr)
		}
		return nil
	})
	if err != nil {
		findings = append(findings, fmt.Errorf("render: walking the tree: %w", err))
	}
	return findings
}

// placesSlots reports whether a parsed template calls the slots or
// slot builtin anywhere: the parse tree is walked rather than the
// text, so a marker in a comment does not count.
func placesSlots(node parse.Node) bool {
	switch n := node.(type) {
	case nil:
		return false
	case *parse.ListNode:
		if n == nil {
			return false
		}
		if slices.ContainsFunc(n.Nodes, placesSlots) {
			return true
		}
	case *parse.ActionNode:
		return placesSlots(n.Pipe)
	case *parse.PipeNode:
		if n == nil {
			return false
		}
		for _, cmd := range n.Cmds {
			for _, arg := range cmd.Args {
				if ident, held := arg.(*parse.IdentifierNode); held {
					if ident.Ident == BuiltinSlots || ident.Ident == BuiltinSlot {
						return true
					}
				}
				if placesSlots(arg) {
					return true
				}
			}
		}
	case *parse.IfNode:
		return placesSlots(n.Pipe) || placesSlots(n.List) || placesSlots(n.ElseList)
	case *parse.RangeNode:
		return placesSlots(n.Pipe) || placesSlots(n.List) || placesSlots(n.ElseList)
	case *parse.WithNode:
		return placesSlots(n.Pipe) || placesSlots(n.List) || placesSlots(n.ElseList)
	}
	return false
}
