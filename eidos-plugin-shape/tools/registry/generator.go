// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"embed"
	"io/fs"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the registry generator.
const ID plugin.ID = "registry"

// WiringTag is the tag of the family of the wiring of the catalog. The Go
// backend spells the files of the two families registry.go and
// registry_wiring.go, and a composition can name them in its layout.
const WiringTag sdk.Tag = "wiring"

// The word of the two families of the generator, the directory of its
// templates, and the space that joins the lines of a claim.
const (
	word         = "registry"
	templatesDir = "templates"
	space        = " "
)

// templateFiles contains the templates of the generated bodies.
//
//go:embed templates
var templateFiles embed.FS

// The Go expressions of package shape and of the directive layer that the
// spellings of a spec map to in the generated table.
var (
	formExprs = map[string]string{
		string(specfront.FormShape):    "FormShape",
		string(specfront.FormMixin):    "FormMixin",
		string(specfront.FormContract): "FormContract",
	}
	typeExprs = map[string]string{
		string(specfront.TypeString):    "TypeString",
		string(specfront.TypeInt):       "TypeInt",
		string(specfront.TypeReference): "TypeReference",
	}
	resolutionExprs = map[string]string{
		string(specfront.ResolveCallableInScope): "ResolveCallableInScope",
		string(specfront.ResolvePackageVar):      "ResolvePackageVar",
		string(specfront.ResolveValueField):      "ResolveValueField",
		string(specfront.ResolveHostParam):       "ResolveHostParam",
		string(specfront.ResolveMemberOnHandle):  "ResolveMemberOnHandle",
		string(specfront.ResolveTypeInScope):     "ResolveTypeInScope",
	}
	sourceExprs = map[string]string{
		string(specfront.SourceInput):  "SourceInput",
		string(specfront.SourceResult): "SourceResult",
	}
	arityExprs = map[string]string{
		string(specfront.ArityOne):      "ArityOne",
		string(specfront.ArityOptional): "ArityOptional",
		string(specfront.ArityMany):     "ArityMany",
		string(specfront.ArityAny):      "ArityAny",
	}
)

// specsData is the payload of the template of Specs. It has one entry for
// each spec, in name order.
type specsData struct {
	Specs []specData
}

// specData is one spec as the template of Specs writes it. Each value is a
// Go expression of package shape or of the directive layer, or a text that
// the template quotes.
type specData struct {
	Name        string
	Form        string
	Doc         string
	Group       string
	Key         string
	ID          string
	Detected    bool
	Documentary bool
	Params      []paramData
	Bindings    []bindingData
	Roles       []roleData
	YieldsTo    []string
}

// paramData is one param as the template of Specs writes it.
type paramData struct {
	Const          string
	Fact           string
	Type           string
	Resolution     string
	Minimum        int64
	HasMinimum     bool
	Required       bool
	Counterexample bool
	Roles          []string
	Excludes       []string
	AlsoOn         []string
	Doc            string
}

// bindingData is one binding as the template of Specs writes it.
type bindingData struct {
	Const string
	Fact  string
	From  string
	Index int64
	Doc   string
}

// roleData is one role as the template of Specs writes it.
type roleData struct {
	Const string
	Arity string
}

// detectionsData is the payload of the template of Detections. It has one
// entry for each detected shape, in the order of precedence.
type detectionsData struct {
	Detections []detectionData
}

// detectionData is one detected shape. Const is the constant of its name
// in package shape, and Func is its detector in package detectors.
type detectionData struct {
	Const string
	Func  string
}

// New returns the registry generator. Its one rule reads every spec of the
// scope of the plan, because the order of the detectors depends on every
// spec. The rule writes two families, one file for the plan each:
//
//   - The primary family is the registry of package shape. It declares the
//     summary keys, and for each spec the constants of its name, its param
//     keys, its binding keys and its roles, its params struct, its reader
//     and the handles of its keys. Its function Specs describes every
//     spec.
//   - The family [WiringTag] is the wiring of package catalog. Its function
//     Detections lists every detected shape and its detector in the order
//     of precedence.
//
// The rule reports two specs of one name and each Go identifier that the
// specs give twice under [specfront.SpecDuplicate], each yields_to entry
// that is no detected shape under [specfront.SpecInvalid], and each cycle
// of yields_to entries under [specfront.PrecedenceCycle], at the spec. It
// writes nothing when it reports. Each call returns a new generator,
// because a plugin instance belongs to one workspace.
func New() plugin.Generator {
	tree, _ := fs.Sub(templateFiles, templatesDir)
	g, _ := sdk.NewPlugin(ID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: word}).
		Output(plugin.Output{Tag: string(WiringTag), Per: plugin.PerPlan, Word: word}).
		Templates(tree).
		Handle(sdk.OnGraph(generate)).
		Build().(plugin.Generator)
	return g
}

// generate is the rule of the generator. It reads the specs, checks them,
// and writes both families.
func generate(m *sdk.GraphMatch, e *sdk.Emitter) error {
	entries := read(m)
	if !check(m, entries) {
		return nil
	}
	out := e.PlanFile()
	out.Append(summaries()...)
	specs := specsData{Specs: make([]specData, 0, len(entries))}
	for _, en := range entries {
		out.Append(declarations(en)...)
		specs.Specs = append(specs.Specs, specOf(en))
	}
	out.Append(&emit.Function{
		Doc: wrap(specsFunc + " returns the description of every spec of the catalog, in the order of the " +
			"names of the specs. Each call returns new slices."),
		Name:       specsFunc,
		Visibility: symbol.VisibilityPublic,
		Returns: []*emit.Return{{Type: &emit.TypeRef{
			Spelling: "[]" + specType, Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: specType}},
		}}},
		Body: emit.Body{Ref: e.Ref(specsTemplate, specs)},
	})
	detections := detectionsData{}
	for _, en := range precedence(entries) {
		detections.Detections = append(detections.Detections, detectionData{
			Const: naming.Pascal(en.name), Func: naming.Pascal(en.name),
		})
	}
	e.PlanFile(WiringTag).Append(&emit.Function{
		Doc: wrap(detectionsFunc + " returns every detected shape and its detector, in the order of " +
			"precedence. A shape comes before every shape that yields to it. Each call returns a new slice."),
		Name:       detectionsFunc,
		Visibility: symbol.VisibilityPublic,
		Returns: []*emit.Return{{Type: &emit.TypeRef{
			Spelling: "[]" + detectionType, Form: symbol.FormList,
			Elems: []*emit.TypeRef{{Spelling: detectionType, Package: CatalogPath}},
		}}},
		Body: emit.Body{Ref: e.Ref(wiringTemplate, detections)},
	})
	return nil
}

// specOf returns one spec as the template of Specs writes it.
func specOf(en entry) specData {
	n := namesOf(en)
	s := specData{
		Name: en.name, Form: formExprs[en.form], Doc: en.doc,
		Group: n.group, Key: n.familyKey, ID: n.idKey,
		Detected: en.detected, Documentary: en.documentary,
	}
	for i, p := range en.params {
		d := paramData{
			Const: n.paramConsts[i], Fact: n.paramKeys[i], Type: typeExprs[p.typ],
			Resolution: resolutionExprs[p.resolve], Required: p.required, Counterexample: p.counterexample,
			Minimum: p.minimum, HasMinimum: p.hasMinimum, Doc: p.doc,
		}
		for _, r := range p.applies {
			d.Roles = append(d.Roles, naming.Pascal(en.name+wordSep+r))
		}
		for _, x := range p.excludes {
			d.Excludes = append(d.Excludes, naming.Pascal(en.name+wordSep+x))
		}
		for _, o := range p.alsoOn {
			d.AlsoOn = append(d.AlsoOn, naming.Pascal(en.name+wordSep+o))
		}
		s.Params = append(s.Params, d)
	}
	for i, b := range en.bindings {
		s.Bindings = append(s.Bindings, bindingData{
			Const: n.bindingConsts[i], Fact: n.bindingKeys[i], From: sourceExprs[b.from],
			Index: b.index, Doc: b.doc,
		})
	}
	for i, r := range en.roles {
		s.Roles = append(s.Roles, roleData{Const: n.roleConsts[i], Arity: arityExprs[r.arity]})
	}
	for _, y := range en.yields {
		s.YieldsTo = append(s.YieldsTo, naming.Pascal(y))
	}
	return s
}
