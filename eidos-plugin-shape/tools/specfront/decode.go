// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"bytes"
	"errors"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/sdk/position"
)

// The directories of the three forms, and the extension of a spec file.
const (
	ShapesDir    = "spec/shapes"
	MixinsDir    = "spec/mixins"
	ContractsDir = "spec/contracts"
	Extension    = ".yaml"
)

// syntaxPrefix is the start of the message of a syntax error of the YAML
// decoder.
const syntaxPrefix = "yaml: "

// textType is the type of a section of free text, such as a claim or the
// doc of a param, which a required section does not leave blank.
var textType = reflect.TypeFor[string]()

// The tags of the document types that the JSON Schema builder reads, and
// that the decoder reads to find the required sections.
const (
	yamlTag     = "yaml"
	schemaTag   = "schema"
	requiredTag = "required"
	tagEnd      = ","
)

// The sections of a document whose positions the checks read.
const (
	keyName            = "name"
	keyParams          = "params"
	keyBindings        = "bindings"
	keyRoles           = "roles"
	keyPrecedence      = "precedence"
	keyCounterexamples = "counterexamples"
)

// spec is one decoded and checked spec, with the position of each part
// that a later fault reports at.
type spec struct {
	file        string
	form        string
	name        string
	at          position.Pos
	claim       string
	detected    bool
	documentary bool
	params      []param
	bindings    []binding
	roles       []role
	yields      []string
}

// param is one param of a spec and its position.
type param struct {
	Param
	at position.Pos
}

// binding is one binding of a shape, its name and its position.
type binding struct {
	Binding
	name string
	at   position.Pos
}

// role is one role of a contract, its name and its position.
type role struct {
	name  string
	arity Arity
	at    position.Pos
}

// decoding is the state of the decode of one spec file. It has the
// function that reports a fault, and whether it reported one.
type decoding struct {
	file   string
	report func(at position.Pos, format string, args ...any)
	failed bool
}

// decode decodes and checks one spec file. It reports every fault through
// report, positioned at the key or the value at fault, and returns false
// for a file with a fault. A file whose structure does not decode reports
// no fault of the checks that read the decoded values.
func decode(file string, data []byte, report func(at position.Pos, format string, args ...any)) (spec, bool) {
	d := &decoding{file: file, report: report}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		d.entries(err)
		return spec{}, false
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		d.fault(d.pos(&doc), "the file is not a mapping of sections to values")
		return spec{}, false
	}
	root := doc.Content[0]
	var s spec
	switch path.Dir(file) {
	case ShapesDir:
		var v Shape
		if !d.strict(data, root, &v) {
			return spec{}, false
		}
		s = spec{
			form: string(FormShape), name: string(v.Name), claim: v.Claim, detected: v.Detected,
			params: d.params(root, v.Params), bindings: d.bindings(root, v.Bindings),
		}
		d.precedence(root, v, &s)
		d.counterexamples(root, v.Counterexamples)
	case MixinsDir:
		var v Mixin
		if !d.strict(data, root, &v) {
			return spec{}, false
		}
		s = spec{
			form: string(FormMixin), name: string(v.Name), claim: v.Claim, documentary: v.Documentary,
			params: d.params(root, v.Params),
		}
		d.counterexamples(root, v.Counterexamples)
	case ContractsDir:
		var v Contract
		if !d.strict(data, root, &v) {
			return spec{}, false
		}
		s = spec{
			form: string(FormContract), name: string(v.Name), claim: v.Claim, documentary: v.Documentary,
			params: d.params(root, v.Params), roles: d.roles(root, v.Roles),
		}
		d.counterexamples(root, v.Counterexamples)
	default:
		d.fault(d.pos(root), "%s is no directory of a form: move the spec to %s, %s or %s",
			path.Dir(file), ShapesDir, MixinsDir, ContractsDir)
		return spec{}, false
	}
	s.file = file
	_, nameNode := value(root, keyName)
	s.at = d.pos(nameNode)
	d.name(s)
	d.checkParams(s)
	if d.failed {
		return spec{}, false
	}
	return s, true
}

// strict decodes data into the document v with unknown keys refused, after
// it reports each required section that the mapping or a nested mapping
// does not contain. It reports each fault of the decoder at its line, and
// reports false where anything failed.
func (d *decoding) strict(data []byte, root *yaml.Node, v any) bool {
	d.required(root, reflect.TypeOf(v).Elem())
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		d.entries(err)
	}
	return !d.failed
}

// required reports each required section that a mapping node does not
// contain, and each required text that is blank, by the tags of the
// struct type t. It descends into the sections whose type is a struct, a
// list of structs or a map of structs. A node of another kind than the
// type is the decoder's fault to report, so required skips it.
func (d *decoding) required(n *yaml.Node, t reflect.Type) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		if n.Kind != yaml.MappingNode {
			return
		}
		for f := range t.Fields() {
			key, _, _ := strings.Cut(f.Tag.Get(yamlTag), tagEnd)
			_, v := value(n, key)
			isRequired := f.Tag.Get(schemaTag) == requiredTag
			switch {
			case v == nil && isRequired:
				d.fault(d.pos(n), "the mapping has no %s, which is required", key)
			case v == nil:
			case isRequired && f.Type == textType && v.Kind == yaml.ScalarNode && strings.TrimSpace(v.Value) == "":
				d.fault(d.pos(v), "the %s is blank: write its text", key)
			default:
				d.required(v, f.Type)
			}
		}
	case reflect.Slice:
		if n.Kind == yaml.SequenceNode {
			for _, e := range n.Content {
				d.required(e, t.Elem())
			}
		}
	case reflect.Map:
		if n.Kind == yaml.MappingNode {
			for i := 1; i < len(n.Content); i += 2 {
				d.required(n.Content[i], t.Elem())
			}
		}
	}
}

// entries reports each fault of a decoder error at its line. A
// *yaml.TypeError has one fault for each entry, and a syntax error has one
// fault, whose message starts with its line.
func (d *decoding) entries(err error) {
	entries := []string{strings.TrimPrefix(err.Error(), syntaxPrefix)}
	if typed, is := errors.AsType[*yaml.TypeError](err); is {
		entries = typed.Errors
	}
	for _, entry := range entries {
		at := position.Pos{File: d.file}
		msg := entry
		if rest, numbered := strings.CutPrefix(entry, linePrefix); numbered {
			digits, text, _ := strings.Cut(rest, lineEnd)
			if line, err := strconv.Atoi(digits); err == nil {
				at.Line, msg = line, text
			}
		}
		d.fault(at, "%s", msg)
	}
}

// name reports a name that is a part of a summary key, and a file whose
// name is not the spec's name, at the spec's name.
func (d *decoding) name(s spec) {
	if slices.ContainsFunc(Summaries, func(sum Summary) bool { return sum.Part == s.name }) {
		d.fault(s.at, "%s is the part of a summary key, such as %s.%s, and no spec can have it",
			s.name, Namespace, s.name)
	}
	if want := s.name + Extension; path.Base(d.file) != want {
		d.fault(s.at, "the spec %s is in %s: name the file %s", s.name, path.Base(d.file), want)
	}
}

// params returns the params of a document with their positions.
func (d *decoding) params(root *yaml.Node, ps []Param) []param {
	if len(ps) == 0 {
		return nil
	}
	_, list := value(root, keyParams)
	out := make([]param, 0, len(ps))
	for i, p := range ps {
		out = append(out, param{Param: p, at: d.pos(list.Content[i])})
	}
	return out
}

// bindings returns the bindings of a shape in the order of the file, with
// their positions, and reports a binding at a negative index.
func (d *decoding) bindings(root *yaml.Node, bs map[Name]Binding) []binding {
	if len(bs) == 0 {
		return nil
	}
	_, m := value(root, keyBindings)
	out := make([]binding, 0, len(bs))
	for i := 0; i+1 < len(m.Content); i += 2 {
		key := m.Content[i]
		b := bs[Name(key.Value)]
		if b.Index < 0 {
			d.fault(d.pos(m.Content[i+1]), "the binding %s has the index %d, and a position counts from 0",
				key.Value, b.Index)
		}
		out = append(out, binding{Binding: b, name: key.Value, at: d.pos(key)})
	}
	return out
}

// roles returns the roles of a contract in the order of the file, with
// their positions, and reports a contract without a role.
func (d *decoding) roles(root *yaml.Node, rs map[Name]RoleArity) []role {
	key, m := value(root, keyRoles)
	if len(rs) == 0 {
		d.fault(d.pos(key), "the contract has no role: list each role of the protocol with its arity")
		return nil
	}
	out := make([]role, 0, len(rs))
	for i := 0; i+1 < len(m.Content); i += 2 {
		name := m.Content[i]
		out = append(out, role{name: name.Value, arity: rs[Name(name.Value)].Arity, at: d.pos(name)})
	}
	return out
}

// precedence reads a shape's precedence into s, and reports precedence on
// a shape that is not detected.
func (d *decoding) precedence(root *yaml.Node, v Shape, s *spec) {
	if v.Precedence == nil {
		return
	}
	if !v.Detected {
		key, _ := value(root, keyPrecedence)
		d.fault(d.pos(key), "the shape %s is not detected, so no detector ranks it: remove its precedence", v.Name)
	}
	for _, y := range v.Precedence.YieldsTo {
		s.yields = append(s.yields, string(y))
	}
}

// counterexamples reports a spec that states no counterexample.
func (d *decoding) counterexamples(root *yaml.Node, c Counterexamples) {
	if c == (Counterexamples{}) {
		key, _ := value(root, keyCounterexamples)
		d.fault(d.pos(key), "the spec has no counterexample: write at least one of invalid, unsafe, edge and refused")
	}
}

// checkParams reports the faults of the params and the bindings of a spec
// that the decoder does not check. They are a key that two params or
// bindings have, a reserved key, and the faults of each param that
// [decoding.checkParam] reports.
func (d *decoding) checkParams(s spec) {
	keys := make(map[string]position.Pos, len(s.params)+len(s.bindings))
	claim := func(key string, at position.Pos) {
		if slices.Contains(ReservedKeys, key) {
			d.fault(at, "%s is reserved: the directive layer or the keys of the catalog use it", key)
		}
		if first, taken := keys[key]; taken {
			d.fault(at, "%s is a key of the spec already, at line %d", key, first.Line)
			return
		}
		keys[key] = at
	}
	byKey := make(map[Name]Param, len(s.params))
	for _, p := range s.params {
		claim(string(p.Key), p.at)
		byKey[p.Key] = p.Param
	}
	for _, b := range s.bindings {
		claim(b.name, b.at)
	}
	for _, p := range s.params {
		d.checkParam(s, p, byKey)
	}
}

// checkParam reports the faults of one param against the other params of
// the spec. They are a reference without a resolution kind, a resolution
// kind on another type, roles outside a contract or outside its roles, a
// minimum on another type than an int, and an entry of excludes or
// also_on that is no fitting param.
func (d *decoding) checkParam(s spec, p param, byKey map[Name]Param) {
	switch {
	case p.Type == TypeReference && p.Resolve == "":
		d.fault(p.at, "the reference %s has no resolve: write the kind of declaration that it refers to", p.Key)
	case p.Type != TypeReference && p.Resolve != "":
		d.fault(p.at, "the %s param %s has resolve %s, and only a reference resolves", p.Type, p.Key, p.Resolve)
	}
	if len(p.Roles) > 0 && s.form != string(FormContract) {
		d.fault(p.at, "the param %s has roles, and only a contract has roles", p.Key)
	} else {
		for _, r := range p.Roles {
			if !slices.ContainsFunc(s.roles, func(c role) bool { return c.name == string(r) }) {
				d.fault(p.at, "the param %s applies under the role %s, and the contract has no such role", p.Key, r)
			}
		}
	}
	if p.Minimum != nil && p.Type != TypeInt {
		d.fault(p.at, "the %s param %s has a minimum, and only an int has one", p.Type, p.Key)
	}
	for _, x := range p.Excludes {
		if _, declared := byKey[x]; !declared || x == p.Key {
			d.fault(p.at, "the param %s excludes %s, which is no other param of the spec", p.Key, x)
		}
	}
	if len(p.AlsoOn) > 0 && p.Resolve != ResolveHostParam {
		d.fault(p.at, "the param %s has also_on, and only a host-param reference refers to a parameter", p.Key)
	}
	for _, o := range p.AlsoOn {
		if q, declared := byKey[o]; !declared || q.Resolve != ResolveCallableInScope {
			d.fault(p.at, "the param %s is also on %s, which is no callable param of the spec", p.Key, o)
		}
	}
}

// fault reports a fault at a position, and marks the decode failed.
func (d *decoding) fault(at position.Pos, format string, args ...any) {
	d.failed = true
	d.report(at, format, args...)
}

// pos returns the position of a node in the file, and the file's own
// position for no node.
func (d *decoding) pos(n *yaml.Node) position.Pos {
	at := position.Pos{File: d.file}
	if n != nil {
		at.Line, at.Col = n.Line, n.Column
	}
	return at
}

// value returns the key node and the value node of a key in a mapping
// node, and two nils where the mapping does not contain the key.
func value(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}
