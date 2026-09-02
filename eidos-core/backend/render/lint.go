// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"text/template"
	"text/template/parse"
)

// Lint holds one plugin's template tree to the static half of the
// template rules, against this pass's target: every template parses
// with the merged vocabulary, so an unknown function fails here
// rather than at execute time; every template carries a slots or
// slot marker, because a body-claiming template that dropped them
// strands contributions at render; a helper shadowing a shared
// name without a declared override is refused the way the render
// refuses it; and a declared override must replace something
// shared. It returns one error per finding and nothing for a
// valid tree. What the lint cannot see is a reference's payload
// and everything inside verbatim, which is the cost those
// carry.
func (p *Pass) Lint(tree fs.FS, funcs template.FuncMap, overrides []string) []error {
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
	for name, fn := range funcs {
		switch {
		case reserved(name):
			findings = append(findings, fmt.Errorf(
				"render: the helper %q claims a builtin's name", name,
			))
		case !declared[name]:
			if _, shared := p.shared[name]; shared {
				findings = append(findings, fmt.Errorf(
					"render: the helper %q shadows %s's shared vocabulary without declaring the override",
					name, p.name,
				))
				continue
			}
			vocabulary[name] = fn
		default:
			vocabulary[name] = fn
		}
	}
	// The stubs mirror exactly what a plugin tree's execution
	// binds — slots, slot and use — so a builtin the render would
	// refuse fails here too, which is this check's whole promise.
	stubs := template.FuncMap{
		BuiltinUse:   func(string) (string, error) { return "", nil },
		BuiltinSlots: func() (string, error) { return "", nil },
		BuiltinSlot:  func(string) (string, error) { return "", nil },
	}

	walk := func(name string) error {
		src, err := fs.ReadFile(tree, name)
		if err != nil {
			return fmt.Errorf("render: reading %s: %w", name, err)
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
