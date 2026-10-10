// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// CatalogPath is the module path of the catalog. The root package of the
// module is package shape.
const CatalogPath = "go.dokimi.dev/eidos/plugin/shape"

// The import paths of the generated code.
const (
	sdkPath       = "go.dokimi.dev/eidos/sdk"
	directivePath = sdkPath + "/directive"
	metaPath      = sdkPath + "/meta"
	symbolPath    = sdkPath + "/symbol"
)

// The spellings of the types of the generated declarations. A type of
// package shape has a bare spelling in the declarations of the package,
// and a type of the SDK facade has the name of its package.
const (
	shapeNameType    = "Shape"
	mixinNameType    = "Mixin"
	contractNameType = "Contract"
	roleType         = "Role"
	specType         = "Spec"
	detectionType    = "shape.Detection"
	matcherType      = "sdk.Matcher"
	paramKeyType     = "directive.ParamKey"
	keyNameType      = "meta.KeyName"
	keyType          = "meta.Key"
	identityType     = "symbol.Identity"
	stringType       = "string"
	boolType         = "bool"
	int64Type        = "int64"
)

// The names of the generated functions, the parameter of a reader, the
// fields of the params of a contract, the template of each body, and the
// word of the constant of a summary key.
const (
	specsFunc      = "Specs"
	detectionsFunc = "Detections"
	readerParam    = "m"
	roleField      = "Role"
	idField        = "ID"
	specsTemplate  = "specs.tmpl"
	wiringTemplate = "detections.tmpl"
	summaryWord    = "key"
)

// The width of a line of generated documentation, the full stop that ends
// a sentence, and the nouns of the owner of a key constant.
const (
	docWidth    = 76
	stop        = "."
	paramNoun   = "param"
	bindingNoun = "binding"
)

// formNouns are the nouns of the forms in generated documentation.
var formNouns = map[string]string{
	string(specfront.FormShape):    "shape",
	string(specfront.FormMixin):    "mixin",
	string(specfront.FormContract): "contract",
}

// summaries returns the constants of the summary keys, in the order of
// [specfront.Summaries].
func summaries() []symbol.Symbol {
	out := make([]symbol.Symbol, 0, len(specfront.Summaries))
	for _, s := range specfront.Summaries {
		name := naming.Pascal(summaryWord + wordSep + s.Part)
		out = append(out, &emit.Constant{
			Doc:        wrap(name + " is the key of " + s.Doc + stop),
			Name:       name,
			Visibility: symbol.VisibilityPublic,
			Type:       &emit.TypeRef{Spelling: keyNameType, Package: metaPath},
			Value:      strconv.Quote(specfront.Namespace + partSep + s.Part),
		})
	}
	return out
}

// declarations returns the declarations of one spec in package shape. They
// are the constant of its name, the constants of its param keys, binding
// keys and roles, its params struct, its reader, and the variables of the
// handles of its keys.
func declarations(e entry) []symbol.Symbol {
	n := namesOf(e)
	noun := formNouns[e.form]
	out := []symbol.Symbol{&emit.Constant{
		Origin:     e.id,
		Doc:        wrap(n.constant + " is the " + noun + " " + e.name + ". " + e.doc),
		Name:       n.constant,
		Visibility: symbol.VisibilityPublic,
		Type:       &emit.TypeRef{Spelling: nameType(e.form)},
		Value:      strconv.Quote(e.name),
	}}
	for i, p := range e.params {
		out = append(out, keyConstant(e, n.paramConsts[i], paramNoun, p.key, p.doc))
	}
	for i, b := range e.bindings {
		out = append(out, keyConstant(e, n.bindingConsts[i], bindingNoun, b.name, b.doc))
	}
	for i, r := range e.roles {
		out = append(out, &emit.Constant{
			Origin:     e.id,
			Doc:        wrap(n.roleConsts[i] + " is the role " + r.name + " of the contract " + e.name + stop),
			Name:       n.roleConsts[i],
			Visibility: symbol.VisibilityPublic,
			Type:       &emit.TypeRef{Spelling: roleType},
			Value:      strconv.Quote(r.name),
		})
	}
	out = append(out, paramsStruct(e, n), reader(e, n))
	return append(out, keyVariables(e, n)...)
}

// nameType returns the type of the constant of the name of a spec, by the
// form of the spec.
func nameType(form string) string {
	switch form {
	case string(specfront.FormMixin):
		return mixinNameType
	case string(specfront.FormContract):
		return contractNameType
	default:
		return shapeNameType
	}
}

// keyConstant returns the constant of the key of a param or a binding,
// typed as a param key of the directive layer. noun is the kind of the
// owner of the key, param or binding, and doc is the meaning of the owner.
func keyConstant(e entry, name, noun, key, doc string) *emit.Constant {
	return &emit.Constant{
		Origin: e.id,
		Doc: wrap(name + " is the key of the " + noun + " " + key + " of the " + formNouns[e.form] + " " +
			e.name + ". The " + noun + " is " + doc + stop),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Type:       &emit.TypeRef{Spelling: paramKeyType, Package: directivePath},
		Value:      strconv.Quote(key),
	}
}

// paramsStruct returns the params struct of a spec. A contract has the
// fields Role and ID first. Every spec has one field for each binding and
// each param, typed as the value of its fact.
func paramsStruct(e entry, n names) *emit.Struct {
	s := &emit.Struct{
		Origin:     e.id,
		Doc:        wrap(n.params + " are the params of the " + formNouns[e.form] + " " + e.name + stop),
		Name:       n.params,
		Visibility: symbol.VisibilityPublic,
	}
	if e.form == string(specfront.FormContract) {
		s.Fields.Append(
			&emit.Field{
				Doc:        wrap(roleField + " is the role of the callable in the instance."),
				Name:       roleField,
				Visibility: symbol.VisibilityPublic,
				Type:       &emit.TypeRef{Spelling: roleType},
			},
			&emit.Field{
				Doc:        wrap(idField + " is the id of the instance, and empty for the one instance of a package."),
				Name:       idField,
				Visibility: symbol.VisibilityPublic,
				Type:       &emit.TypeRef{Spelling: stringType},
			},
		)
	}
	for _, b := range e.bindings {
		field := naming.Pascal(b.name)
		s.Fields.Append(&emit.Field{
			Doc:        wrap(field + " is " + b.doc + stop),
			Name:       field,
			Visibility: symbol.VisibilityPublic,
			Type:       &emit.TypeRef{Spelling: identityType, Package: symbolPath},
		})
	}
	for _, p := range e.params {
		field := naming.Pascal(p.key)
		s.Fields.Append(&emit.Field{
			Doc:        wrap(field + " is " + p.doc + stop),
			Name:       field,
			Visibility: symbol.VisibilityPublic,
			Type:       valueType(p.typ),
		})
	}
	return s
}

// reader returns the reader of a spec. The reader is the function that
// reads the classification and its values on the subject of a match.
func reader(e entry, n names) *emit.Function {
	noun := formNouns[e.form]
	var doc string
	var body strings.Builder
	body.WriteString("var p " + n.params + "\n")
	if e.form == string(specfront.FormContract) {
		doc = n.reader + " returns the role, the instance id and the params of the contract " + e.name +
			" on the subject of m, and false where the subject has no role in the contract."
		body.WriteString("role, held := sdk.Fact(m, " + n.familyVar + ")\n")
		body.WriteString("if !held {\nreturn p, false\n}\n")
		body.WriteString("p." + roleField + " = " + roleType + "(role)\n")
		body.WriteString("p." + idField + ", _ = sdk.Fact(m, " + n.idVar + ")\n")
	} else {
		doc = n.reader + " returns the params of the " + noun + " " + e.name +
			" on the subject of m, and false where the subject does not have the " + noun + "."
		body.WriteString("if _, held := sdk.Fact(m, " + n.familyVar + "); !held {\nreturn p, false\n}\n")
	}
	for i, b := range e.bindings {
		body.WriteString("p." + naming.Pascal(b.name) + ", _ = sdk.Fact(m, " + n.bindingVars[i] + ")\n")
	}
	for i, p := range e.params {
		body.WriteString("p." + naming.Pascal(p.key) + ", _ = sdk.Fact(m, " + n.paramVars[i] + ")\n")
	}
	body.WriteString("return p, true\n")
	return &emit.Function{
		Origin:     e.id,
		Doc:        wrap(doc + " It records each read in the invocation of m."),
		Name:       n.reader,
		Visibility: symbol.VisibilityPublic,
		Params: []*emit.Param{{
			Name: readerParam,
			Type: &emit.TypeRef{Spelling: matcherType, Package: sdkPath},
		}},
		Returns: []*emit.Return{
			{Type: &emit.TypeRef{Spelling: n.params}},
			{Type: &emit.TypeRef{Spelling: boolType}},
		},
		Body: emit.Body{Verbatim: body.String()},
	}
}

// keyVariables returns the variables of the handles of the keys of a
// spec. They are the family key, the id key of a contract, and the key of
// each binding and each param. Each handle has the name of its key, and
// the plugin shape registers the key.
func keyVariables(e entry, n names) []symbol.Symbol {
	familyType := boolType
	if e.form == string(specfront.FormContract) {
		familyType = stringType
	}
	vars := []symbol.Symbol{keyVariable(e, n.familyVar, n.familyKey, familyType)}
	if n.idVar != "" {
		vars = append(vars, keyVariable(e, n.idVar, n.idKey, stringType))
	}
	for i := range e.bindings {
		vars = append(vars, keyVariable(e, n.bindingVars[i], n.bindingKeys[i], identityType))
	}
	for i, p := range e.params {
		vars = append(vars, keyVariable(e, n.paramVars[i], n.paramKeys[i], valueType(p.typ).Spelling))
	}
	return vars
}

// keyVariable returns the variable of the handle of one key of a spec.
// typ is the spelling of the Go type of the fact of the key.
func keyVariable(e entry, name, key, typ string) *emit.Variable {
	return &emit.Variable{
		Origin:     e.id,
		Doc:        wrap(name + " is the handle of the key " + key + stop),
		Name:       name,
		Visibility: symbol.VisibilityPackage,
		Type: &emit.TypeRef{
			Spelling: keyType, Package: metaPath,
			Args: []*emit.TypeRef{valueRef(typ)},
		},
		Value: "meta.Named[" + typ + "](" + strconv.Quote(key) + ")",
	}
}

// valueType returns the reference to the Go type of the fact of a param,
// by the type of the param. An int is an int64, and a reference is an
// identity.
func valueType(typ string) *emit.TypeRef {
	switch specfront.ParamType(typ) {
	case specfront.TypeInt:
		return valueRef(int64Type)
	case specfront.TypeReference:
		return valueRef(identityType)
	default:
		return valueRef(stringType)
	}
}

// valueRef returns a new reference to the Go type of a fact, by the
// spelling of the type. The reference to an identity imports the symbol
// package of the SDK facade.
func valueRef(spelling string) *emit.TypeRef {
	if spelling == identityType {
		return &emit.TypeRef{Spelling: identityType, Package: symbolPath}
	}
	return &emit.TypeRef{Spelling: spelling}
}

// wrap splits a text into lines of documentation of at most docWidth
// bytes, at the spaces between its words. A word longer than the width
// has a line of its own.
func wrap(text string) []string {
	var lines []string
	var line strings.Builder
	for word := range strings.FieldsSeq(text) {
		if line.Len() > 0 && line.Len()+len(space)+len(word) > docWidth {
			lines = append(lines, line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteString(space)
		}
		line.WriteString(word)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}
