// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/declarative"
)

// Memo configures the parse memo.
type Memo struct {
	Limit Bytes  `yaml:"limit" doc:"The size of the memo in bytes. The value is a number, or a number followed by KiB, MiB or GiB. The memo is off when the limit is 0."`
	Dir   string `yaml:"dir"   doc:"The directory of the memo entries. A relative path is relative to the workspace root. The default is the memo directory inside the state directory."`
}

// Sources is the source scope of a plan. It has the fields of
// [workspace.Sources].
type Sources struct {
	Lang     symbol.Lang `yaml:"lang"     doc:"The source language of the packages, such as golang."`
	Packages []string    `yaml:"packages" doc:"The package directories. A directory is relative to the workspace root and uses slashes. ./svc is one directory, and ./svc/... is that directory and every directory below it."`
	Module   string      `yaml:"module"   doc:"The module path of the packages."`
}

// Layout contains the fields of [layout.Config] that a config file can set.
type Layout struct {
	Policy     Policy                        `yaml:"policy"     doc:"The default placement of a generated file. The value is inherit, alongside-source or centralised."`
	Dir        string                        `yaml:"dir"        doc:"The output directory of the plan. The path is relative to the workspace root and uses slashes."`
	ImportBase string                        `yaml:"importBase" doc:"The import path of the output directory. A plan needs it when no loaded module contains the output directory."`
	Generators map[plugin.ID]GeneratorLayout `yaml:"generators" doc:"The refinements of the routing of the generators of the plan. Each key is the name of a generator of the plan."`
}

// GeneratorLayout is the refinement of the routing of one generator and of
// its families. Each field that the file sets replaces the same field of the
// refinement that the composition declares for the generator.
type GeneratorLayout struct {
	Policy   Policy                  `yaml:"policy"   doc:"The placement of the files of the generator. The value is inherit, alongside-source or centralised."`
	Dir      string                  `yaml:"dir"      doc:"The output directory of the files of the generator. The path is relative to the workspace root and uses slashes."`
	File     string                  `yaml:"file"     doc:"The filename of every file of the generator, in place of the filename that the target spells."`
	Families map[string]FamilyLayout `yaml:"families" doc:"The refinements of the routing of the families of the generator. Each key is the tag of a family, and the empty key names the primary family."`
}

// FamilyLayout is the refinement of the routing of one family of a
// generator. Each field that the file sets replaces the same field of the
// refinement that the composition declares for the family.
type FamilyLayout struct {
	Policy Policy `yaml:"policy" doc:"The placement of the files of the family. The value is inherit, alongside-source or centralised."`
	Dir    string `yaml:"dir"    doc:"The output directory of the files of the family. The path is relative to the workspace root and uses slashes."`
	File   string `yaml:"file"   doc:"The filename of every file of the family, in place of the filename that the target spells."`
}

// Plan is the refinement of one plan of the binary. Build leaves a value of
// the plan unchanged when the matching field is nil.
type Plan struct {
	// Enabled is nil when the file does not set it. The plan runs when
	// Enabled is nil or true.
	Enabled  *bool                              `yaml:"enabled"  doc:"When false, the composition leaves the plan out, and the next run removes the files of the plan. The default is true."`
	Sources  *Sources                           `yaml:"sources"  doc:"The source packages of the plan. They replace the sources that the plan declares."`
	Layout   *Layout                            `yaml:"layout"   doc:"The policy, the output directory, the import base and the refinements of the generators of the plan. Each value that the file sets replaces the value of the plan."`
	Policies map[plugin.PolicyKey]plugin.Choice `yaml:"policies" doc:"The choices of the lowering policies of the plan's backend. Each key is a policy key, such as typescript.int64, and each value is one of the choices of the key. A choice replaces the choice of the same key at the top level for the plan. Quote a choice that YAML reads as another type, such as \"null\"."`
	Plugins  []string                           `yaml:"plugins"  doc:"The directories of the declarative plugins that the plan runs after the generators of the binary. A directory is absolute or relative to the workspace root, and uses slashes. Build refuses plugins for a plan that the binary does not mark extensible."`
}

// refinement converts the plan into the [workspace.PlanConfig] that Build
// applies. A generator whose entry sets families alone refines no field of
// the generator itself.
func (p Plan) refinement() workspace.PlanConfig {
	out := workspace.PlanConfig{Disabled: p.Enabled != nil && !*p.Enabled, Policies: p.Policies}
	if p.Sources != nil {
		out.Sources = &workspace.Sources{Lang: p.Sources.Lang, Packages: p.Sources.Packages, Module: p.Sources.Module}
	}
	if p.Layout == nil {
		return out
	}
	out.Policy, out.Dir, out.ImportBase = layout.Policy(p.Layout.Policy), p.Layout.Dir, p.Layout.ImportBase
	for name, g := range p.Layout.Generators {
		if r := (layout.Refinement{Policy: layout.Policy(g.Policy), Dir: g.Dir, File: g.File}); r != (layout.Refinement{}) {
			if out.Plugins == nil {
				out.Plugins = make(map[plugin.ID]layout.Refinement, len(p.Layout.Generators))
			}
			out.Plugins[name] = r
		}
		for tag, f := range g.Families {
			if out.Families == nil {
				out.Families = map[layout.Family]layout.Refinement{}
			}
			out.Families[layout.Family{Plugin: name, Tag: tag}] = layout.Refinement{
				Policy: layout.Policy(f.Policy), Dir: f.Dir, File: f.File,
			}
		}
	}
	return out
}

// Document is a config file that configures one workspace. It refines the
// composition of the binary. Every key except version is optional, and the
// composition keeps its own value for a key that the document does not set.
type Document struct {
	Version   Version `yaml:"version"   schema:"required" doc:"The version of the file format. It must be 1."`
	Workspace string  `yaml:"workspace"                   doc:"The name of the workspace. The default is the base name of the workspace root."`
	// Workers is nil when the file does not set it. The composition then
	// keeps its own worker count.
	Workers *Count           `yaml:"workers" doc:"The number of matches that one phase call runs at the same time. With 0 or 1, a phase call runs one match at a time."`
	Ignore  []directive.Name `yaml:"ignore"  doc:"The directive names that the load does not report as unclaimed. A name is a full directive name, or a plugin prefix that ends with a colon."`
	// Memo is nil when the file does not set it. The composition then keeps
	// its own memo.
	Memo     *Memo                              `yaml:"memo"     doc:"The parse memo. It stores the regions of each unit that a run parsed."`
	Plans    map[string]Plan                    `yaml:"plans"    doc:"The refinements of the plans of the binary. Each key is the name of a plan."`
	Policies map[plugin.PolicyKey]plugin.Choice `yaml:"policies" doc:"The choices of the lowering policies. Each key is a policy key, such as typescript.int64, and each value is one of the choices of the key. A choice applies to every plan whose backend declares the key. Quote a choice that YAML reads as another type, such as \"null\"."`
	Options  map[string]map[string]any          `yaml:"options"  doc:"The option values of the plugins. Each key is the name of a plugin, and each nested key is the key of an option."`
	// Extensions contains the settings of the commands that the binary adds.
	// The kernel commands do not read them, and [Extension] decodes one of
	// them strictly.
	Extensions map[string]any `yaml:"extensions" doc:"The settings of the commands that the binary adds. Each key is the name of an extension, and the kernel commands do not read the values."`
}

// Apply sets the values of the document on b. root is the workspace root of
// the composition. Apply changes only the values whose keys the document
// sets.
//
//   - Apply passes workspace to [workspace.Builder.Workspace], and workers
//     to [workspace.Builder.Parallel].
//   - Apply passes ignore to [workspace.Builder.Ignore]. Ignore adds the
//     names to the names that the composition already ignores.
//   - Apply passes memo to [workspace.Builder.Memo], which replaces the memo
//     of the composition. Apply joins a relative memo directory to root, and
//     the workspace opens the memo ledger in that directory with
//     [ledger.OpenAt].
//   - Apply passes plans, policies and options to one call of
//     [workspace.Builder.Config], which replaces the config of the
//     composition.
//   - Apply loads the plugin directories of each plan with
//     [declarative.Load], and passes the generators to the plan as its
//     [workspace.PlanConfig.Generators]. Apply joins a relative directory to
//     root. It loads each directory once, so a directory in two plans is one
//     plugin in both plans.
//
// Error modes: Apply returns an error that joins the errors of the loads, in
// the order of the plan names, and nil when every directory loads. It sets
// the other values on b in both cases.
func (d *Document) Apply(b *workspace.Builder, root string) error {
	if d.Workspace != "" {
		b.Workspace(d.Workspace)
	}
	if d.Workers != nil {
		b.Parallel(int(*d.Workers))
	}
	if len(d.Ignore) > 0 {
		b.Ignore(d.Ignore...)
	}
	if d.Memo != nil {
		m := workspace.Memo{Limit: int64(d.Memo.Limit)}
		if d.Memo.Dir != "" {
			dir := filepath.FromSlash(d.Memo.Dir)
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(root, dir)
			}
			m.Open = func() (ledger.Ledger, error) { return ledger.OpenAt(dir) }
		}
		b.Memo(m)
	}
	if len(d.Plans) == 0 && len(d.Policies) == 0 && len(d.Options) == 0 {
		return nil
	}
	cfg := workspace.Config{Options: d.Options, Policies: d.Policies}
	var errs []error
	if len(d.Plans) > 0 {
		cfg.Plans = make(map[string]workspace.PlanConfig, len(d.Plans))
		loaded := make(map[string]plugin.Generator)
		for _, name := range slices.Sorted(maps.Keys(d.Plans)) {
			p := d.Plans[name]
			pc := p.refinement()
			for _, dir := range p.Plugins {
				dir = filepath.FromSlash(dir)
				if !filepath.IsAbs(dir) {
					dir = filepath.Join(root, dir)
				}
				dir = filepath.Clean(dir)
				g, seen := loaded[dir]
				if !seen {
					var err error
					if g, err = declarative.Load(dir); err != nil {
						errs = append(errs, err)
					}
					loaded[dir] = g
				}
				if g != nil {
					pc.Generators = append(pc.Generators, g)
				}
			}
			cfg.Plans[name] = pc
		}
	}
	b.Config(cfg)
	return errors.Join(errs...)
}
