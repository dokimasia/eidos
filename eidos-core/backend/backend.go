// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// Builder accumulates a backend declaration: the identity and
// target the plan validates, the comment syntax the output
// contract stamps through, and the language pieces the render
// pass varies in. Everything on it is data except the language
// functions; Build freezes it, and a Builder is not reused
// afterwards.
type Builder struct {
	name    plugin.ID
	target  plugin.Target
	syntax  plugin.CommentSyntax
	version string
	lang    render.Language
	// vocab lists the declared vocabulary parts in declaration
	// order, and helpers records every name they declare, so a name
	// declared twice is a defect before any render.
	vocab      []func(set *render.ImportSet) template.FuncMap
	helpers    map[string]bool
	lower      plugin.Lower
	respell    plugin.Respell
	member     plugin.MemberRespell
	packages   plugin.PackageRule
	scopes     func(pkg symbol.Identity) symbol.Identity
	overloads  bool
	types      func(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error)
	supertypes func(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error)
	policies   []plugin.PolicySpec
	defects    []string
}

// New starts a backend declaration for one target.
func New(
	name plugin.ID, target plugin.Target, syntax plugin.CommentSyntax,
) *Builder {
	return &Builder{
		name: name, target: target, syntax: syntax,
		lang: render.Language{
			Kinds:   map[symbol.Kind]string{},
			Refused: map[symbol.Kind]string{},
			Groups:  map[render.GroupName]string{},
		},
		helpers: map[string]bool{},
	}
}

// Version sets the version the composition fingerprint folds in:
// bump it with every change to the rendered output, because work
// keyed without it survives a backend change with nothing
// reporting why.
func (b *Builder) Version(v string) *Builder {
	b.version = v
	return b
}

// FileTemplate sets the file skeleton; undeclared, the pass's
// default renders the import block then the declarations. The
// skeleton executes over the data that [render.Language.File]
// describes. The clause a language opens its files with is the
// skeleton's own to spell; the header is not, because the output
// contract prepends it after the formatter ran.
func (b *Builder) FileTemplate(t string) *Builder {
	b.lang.File = t
	return b
}

// KindTemplates declares how the language spells emit kinds, keyed
// by kind; repeatable, merging, one spelling per kind. Which kinds
// render standalone and which render inside their hosts is the
// language's own split. A kind the language cannot spell is
// declared through [Builder.RefusedKinds], and the conformance
// suite's every-kind check reports a kind a backend neither spells
// nor refuses.
func (b *Builder) KindTemplates(ts map[symbol.Kind]string) *Builder {
	for _, k := range slices.Sorted(maps.Keys(ts)) {
		if _, taken := b.lang.Kinds[k]; taken {
			b.defects = append(b.defects, "spells the "+k.String()+" kind twice")
			continue
		}
		b.lang.Kinds[k] = ts[k]
	}
	return b
}

// RefusedKinds declares the emit kinds the language cannot spell,
// keyed by kind, each with the reason stated as a fact of the
// language; repeatable, merging, one refusal per kind. The render
// skips a declaration of a refused kind and reports the reason
// under [render.RefusedKind]. A kind the language lowers into other
// kinds before any template runs is neither spelt nor refused. The
// built backend returns the refusals through [render.Refuser].
func (b *Builder) RefusedKinds(rs map[symbol.Kind]string) *Builder {
	for _, k := range slices.Sorted(maps.Keys(rs)) {
		if _, taken := b.lang.Refused[k]; taken {
			b.defects = append(b.defects, "refuses the "+k.String()+" kind twice")
			continue
		}
		b.lang.Refused[k] = rs[k]
	}
	return b
}

// Scaffold sets the language's statement printer: how each kind of
// the neutral scaffolding vocabulary spells, recording into the
// file's import set whatever it qualifies with. The body builtin
// calls it for slot contributions and scaffold content alike.
func (b *Builder) Scaffold(
	f func(s emit.Stmt, set *render.ImportSet) ([]byte, error),
) *Builder {
	b.lang.Scaffold = f
	return b
}

// Value sets the language's writer of values: how one value of the
// value tree spells, recording into the file's import set the imports
// of its references and its callees. The value builtin calls it. A
// backend that declares none fails every call of the builtin.
func (b *Builder) Value(
	f func(v emit.Value, set *render.ImportSet) (string, error),
) *Builder {
	b.lang.Value = f
	return b
}

// MemberIndent sets the indentation of a member's statements in the
// body of its type, which the memberbody builtin writes before each
// line of a member's content. A language that leaves it unset places
// members through the body builtin alone.
func (b *Builder) MemberIndent(indent string) *Builder {
	b.lang.MemberIndent = indent
	return b
}

// Funcs registers part of the language's shared template vocabulary,
// which a declared override can replace name by name. The part is a
// function returning the helpers bound to one file's import set, so a helper that spells a type
// records the import the spelling needs. It is repeatable, the parts
// merging, one helper per name. The kit calls each part once here,
// with a set of its own, to read the names it declares. A plugin's
// declared override replaces one of these names for every template
// in the pass.
func (b *Builder) Funcs(part func(set *render.ImportSet) template.FuncMap) *Builder {
	if part == nil {
		b.defects = append(b.defects, "declares a nil vocabulary")
		return b
	}
	for _, name := range slices.Sorted(maps.Keys(part(&render.ImportSet{}))) {
		if b.helpers[name] {
			b.defects = append(b.defects,
				"declares the "+strconv.Quote(name)+" helper twice")
			continue
		}
		b.helpers[name] = true
	}
	b.vocab = append(b.vocab, part)
	return b
}

// Naming sets the target's filename spelling, which the built
// backend serves to the plan's layout as [plugin.FileSpeller].
func (b *Builder) Naming(n render.Naming) *Builder {
	b.lang.Naming = n
	return b
}

// Split sets the target's unit reshaping, served beside the naming.
// Undeclared, every unit files whole.
func (b *Builder) Split(s render.Split) *Builder {
	b.lang.Split = s
	return b
}

// Packages sets the target's package rule: the package a file at a
// routed path declares, which is the package the language's frontend
// names when it reads that file. The built backend implements
// [plugin.Packager] through it. Undeclared, every routed file takes
// the package of its first unit.
func (b *Builder) Packages(r plugin.PackageRule) *Builder {
	b.packages = r
	return b
}

// NameScopes sets the target's namespace rule: the namespace of the
// file-level names of a file that declares a package, which the built
// backend serves as [plugin.NameScoper]. Undeclared, the namespace of a
// file is its package.
func (b *Builder) NameScopes(scope func(pkg symbol.Identity) symbol.Identity) *Builder {
	b.scopes = scope
	return b
}

// Overloads declares that the language of the target declares the
// overloads of a function: two functions of one name in one module, told
// apart by their parameters. The built backend serves the declaration as
// [plugin.Overloader]. Undeclared, the target does not overload, and two
// functions of one name in one routed scope collide.
func (b *Builder) Overloads() *Builder {
	b.overloads = true
	return b
}

// Cluster sets the target's declaration clustering. The group
// templates its clusters select arrive through [Builder.Groups],
// and a cluster without them is a defect at Build.
func (b *Builder) Cluster(c render.Cluster) *Builder {
	b.lang.Cluster = c
	return b
}

// Groups declares the group templates a Cluster selects, keyed by
// group name; repeatable, merging, one spelling per group.
func (b *Builder) Groups(gs map[render.GroupName]string) *Builder {
	for _, g := range slices.Sorted(maps.Keys(gs)) {
		if _, taken := b.lang.Groups[g]; taken {
			b.defects = append(b.defects,
				"spells the "+string(g)+" group twice")
			continue
		}
		b.lang.Groups[g] = gs[g]
	}
	return b
}

// Imports sets the renderer for a file's collected import set:
// grouping and sorting are language facts.
func (b *Builder) Imports(r func(set *render.ImportSet) string) *Builder {
	b.lang.Imports = r
	return b
}

// TemplateImport declares the binding of an import that a template
// states through the use builtin, as [render.Language.TemplateImport]
// describes it. Without it, a template's import does not bind a name in
// the name assignment of the file.
func (b *Builder) TemplateImport(fn func(path, name string) (local, item string, ok bool)) *Builder {
	b.lang.TemplateImport = fn
	return b
}

// Finalise sets the language formatter, run last per file. A
// failure at render is a positioned finding that names the file it
// could not format, and the pass continues with the remaining
// files.
func (b *Builder) Finalise(f func(src []byte) ([]byte, error)) *Builder {
	b.lang.Finalise = f
	return b
}

// Coverage declares the language's fact coverage: one verdict per
// fact, with per-kind exceptions. It arms the render's guard, and
// the built backend returns it through [render.Coverer], which the
// conformance suite reads to check the declaration total and the
// rendered findings.
func (b *Builder) Coverage(c render.Coverage) *Builder {
	b.lang.Coverage = c
	return b
}

// Lower sets the construct lowering the settle applies: one
// declaration in, the target's declaration shapes out, before any
// name respells. The built backend lowers through it, and its Render
// refuses an unsettled store.
func (b *Builder) Lower(l plugin.Lower) *Builder {
	b.lower = l
	return b
}

// Respell sets the name convention the settle applies: every
// declared name spells through it, and references follow. The built
// backend respells through it, and its Render refuses an unsettled
// store.
func (b *Builder) Respell(r plugin.Respell) *Builder {
	b.respell = r
	return b
}

// RespellMember sets the name convention of a member, which reads the level
// and the mutability of the member as well: every field and every method
// spells through it in place of the convention of [Builder.Respell]. The
// built backend serves it as [plugin.MemberRespeller], and its Render refuses
// an unsettled store.
func (b *Builder) RespellMember(r plugin.MemberRespell) *Builder {
	b.member = r
	return b
}

// Types declares the target's spoke, which the built backend serves as
// [plugin.TypeSpeller]: the spelling of the canonical shape of a type
// that another language declares, under a plan's resolved policy.
func (b *Builder) Types(spell func(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error)) *Builder {
	b.types = spell
	return b
}

// Supertypes declares the target's spelling of a supertype of another
// language, which the built backend serves as [plugin.SupertypeSpeller]:
// the reference of a shape that a declaration extends, implements or
// embeds, or that bounds a type parameter. A target without it spells a
// supertype as the spoke of [Builder.Types] spells a value.
func (b *Builder) Supertypes(spell func(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error)) *Builder {
	b.supertypes = spell
	return b
}

// Policies declares the target's lowering policies, which the built
// backend serves as [plugin.PolicyProvider]. It is repeatable, and the
// specs accumulate. Build checks them as [plugin.NewPolicy] does.
func (b *Builder) Policies(specs ...plugin.PolicySpec) *Builder {
	b.policies = append(b.policies, specs...)
	return b
}

// Backend is a backend that the kit built. It is the backend of the
// SPI, with the fact coverage that the guard of the render reads, the
// kinds that the language refuses, which the backend suite checks, the
// spelling of a supertype of another language, the namespace of the
// names of a file, whether the language overloads a function, and the
// convention of a member.
type Backend interface {
	plugin.Backend
	render.Coverer
	render.Refuser
	plugin.SupertypeSpeller
	plugin.NameScoper
	plugin.Overloader
	plugin.MemberRespeller
}

// Build freezes the declaration and returns the lowered backend. Its
// Render is the render pass over the declared language and nothing
// more, so a kit backend and a hand-rolled pass over the same language
// return the same bytes. A method whose part the backend does not
// declare behaves as follows:
//
//   - Lower keeps every declaration;
//   - Respell returns every name unchanged;
//   - RespellMember spells as Respell does;
//   - SpellType returns an error that the target does not spell the
//     types of other languages;
//   - SpellSupertype spells as SpellType does;
//   - PackageAt returns the origin package of the file;
//   - NameScope returns the package that it is given;
//   - Overloads reports false;
//   - Policies returns nil.
//
// Build panics on a declaration defect, because a wrong declaration
// is a bug in the backend's own constructor, and the panic comes on
// the first Build in any test. It checks these groups in order and
// panics at the first group with a defect, naming every defect of
// that group:
//
//   - an empty name;
//   - a zero target;
//   - a kind spelt twice, a kind refused twice, a group spelt twice,
//     a helper declared twice and a nil vocabulary part;
//   - every defect of a policy spec that [plugin.NewPolicy] reports;
//   - every fault [render.New] joins, such as an empty kind-template
//     set, a kind both spelt and refused, a template that does not
//     parse and a missing formatter.
func (b *Builder) Build() Backend {
	if b.name == "" {
		panic("backend: New with an empty name")
	}
	name := string(b.name)
	if b.target == "" {
		panic("backend: " + name + " declares no target")
	}
	if len(b.defects) > 0 {
		panic("backend: " + name + " " + strings.Join(b.defects, ", and "))
	}
	if _, err := plugin.NewPolicy(b.target, b.policies, nil); err != nil {
		panic("backend: " + name + " declares a defective policy:\n" + err.Error())
	}
	b.lang.Funcs = vocabulary(b.vocab)
	pass, err := render.New(b.name, b.lang)
	if err != nil {
		panic("backend: " + name + " declares a language the pass refuses:\n" +
			err.Error())
	}
	return &builtBackend{
		name: b.name, target: b.target, syntax: b.syntax,
		version: b.version, pass: pass, packages: b.packages, scopes: b.scopes, overloads: b.overloads,
		lower: b.lower, respell: b.respell, member: b.member,
		types: b.types, supertypes: b.supertypes, policies: b.policies,
	}
}

// builtBackend is a lowered backend declaration: the roles the
// plan validates as data, and the composed pass Render lowers to.
type builtBackend struct {
	name    plugin.ID
	target  plugin.Target
	syntax  plugin.CommentSyntax
	version string
	pass    *render.Pass
	// packages is the declared package rule, and scopes the declared
	// namespace rule, each nil where the backend declares none. overloads
	// reports that the language overloads a function.
	packages  plugin.PackageRule
	scopes    func(pkg symbol.Identity) symbol.Identity
	overloads bool
	// lower, respell and member are the declared hooks of the settle, nil
	// where the backend declares none. Render refuses an unsettled store
	// where the backend declares any of them.
	lower   plugin.Lower
	respell plugin.Respell
	member  plugin.MemberRespell
	// types is the declared spoke, nil where the backend declares none,
	// and supertypes is the declared spelling of a supertype, nil where the
	// backend declares none.
	types      func(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error)
	supertypes func(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error)
	// policies are the declared lowering policies, in declaration
	// order.
	policies []plugin.PolicySpec
}

var _ Backend = (*builtBackend)(nil)

// Name returns the backend's one identity.
func (b *builtBackend) Name() plugin.ID { return b.name }

// Version implements [plugin.Versioned]: the declared version, ""
// where none was.
func (b *builtBackend) Version() string { return b.version }

// Target returns the target the backend's plan resolves at
// composition.
func (b *builtBackend) Target() plugin.Target { return b.target }

// Syntax returns the language's comment forms for the output
// contract: the generated-file header is written through them,
// after the formatter ran.
func (b *builtBackend) Syntax() plugin.CommentSyntax { return b.syntax }

// Coverage implements [render.Coverer] through the composed pass,
// so the suite reads the same data the render's guard does.
func (b *builtBackend) Coverage() render.Coverage { return b.pass.Coverage() }

// RefusedKinds implements [render.Refuser] through the composed
// pass, so the suite reads the refusals the render reports.
func (b *builtBackend) RefusedKinds() map[symbol.Kind]string { return b.pass.RefusedKinds() }

// SplitUnit implements [plugin.FileSpeller] through the declared
// Split.
func (b *builtBackend) SplitUnit(u plugin.Unit) []plugin.Unit { return b.pass.SplitUnit(u) }

// FileName implements [plugin.FileSpeller] through the declared
// Naming.
func (b *builtBackend) FileName(u plugin.Unit) string { return b.pass.FileName(u) }

// SpellType implements [plugin.TypeSpeller] through the declared spoke.
// A backend without a spoke refuses every shape. Its error reports that
// the target does not spell the types of other languages.
func (b *builtBackend) SpellType(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	if b.types == nil {
		return nil, errors.New("backend: target " + string(b.target) + " does not spell the types of other languages")
	}
	return b.types(s, p)
}

// SpellSupertype implements [plugin.SupertypeSpeller] through the declared
// spelling of a supertype, and through [builtBackend.SpellType] where the
// backend declares none.
func (b *builtBackend) SpellSupertype(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	if b.supertypes == nil {
		return b.SpellType(s, p)
	}
	return b.supertypes(s, p)
}

// Policies implements [plugin.PolicyProvider]: the declared specs, and
// nil where the backend declares none.
func (b *builtBackend) Policies() []plugin.PolicySpec { return b.policies }

// PackageAt implements [plugin.Packager] through the declared package
// rule, and returns the file's origin package where the backend
// declares none.
func (b *builtBackend) PackageAt(p plugin.Placement) (symbol.Identity, error) {
	if b.packages == nil {
		return p.Origin, nil
	}
	return b.packages(p)
}

// NameScope implements [plugin.NameScoper] through the declared namespace
// rule, and returns pkg where the backend declares none.
func (b *builtBackend) NameScope(pkg symbol.Identity) symbol.Identity {
	if b.scopes == nil {
		return pkg
	}
	return b.scopes(pkg)
}

// Overloads implements [plugin.Overloader]: whether the backend declared
// that its language overloads a function.
func (b *builtBackend) Overloads() bool { return b.overloads }

// Render implements [plugin.Renderer] through the composed pass. A
// backend declaring a seam refuses an unsettled store: the plan
// never settled, and rendering it would write the wrong bytes
// without a finding.
func (b *builtBackend) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	seams := b.lower != nil || b.respell != nil || b.member != nil
	if seams && ctx != nil && ctx.Emit != nil && !ctx.Emit.Settled() {
		return nil, errors.New("backend: " + string(b.name) +
			" declares a lowering seam, and the store is unsettled: " +
			"the plan settles once before the render")
	}
	return b.pass.Render(ctx)
}

// Lower implements [plugin.Lowerer] through the declared hook. A
// backend without the hook returns nil and no error, which keeps the
// declaration, and allocates nothing.
func (b *builtBackend) Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	if b.lower == nil {
		return nil, nil
	}
	return b.lower(s)
}

// Respell implements [plugin.Respeller] through the declared hook. A
// backend without the hook returns the name unchanged, and allocates
// nothing.
func (b *builtBackend) Respell(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
	if b.respell == nil {
		return name, nil
	}
	return b.respell(host, kind, v, name)
}

// RespellMember implements [plugin.MemberRespeller] through the declared hook
// of a member, and through [builtBackend.Respell] where the backend declares
// none. It allocates what the hook allocates.
func (b *builtBackend) RespellMember(m plugin.Member, name string) (string, error) {
	if b.member == nil {
		return b.Respell(m.Host, m.Kind, m.Visibility, name)
	}
	return b.member(m, name)
}

// vocabulary joins the declared parts into the one vocabulary the
// pass binds per file, nil where none was declared.
func vocabulary(parts []func(set *render.ImportSet) template.FuncMap) func(*render.ImportSet) template.FuncMap {
	if len(parts) == 0 {
		return nil
	}
	return func(set *render.ImportSet) template.FuncMap {
		out := template.FuncMap{}
		for _, part := range parts {
			maps.Copy(out, part(set))
		}
		return out
	}
}
