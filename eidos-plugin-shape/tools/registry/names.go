// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The words that the generator joins onto the name of a spec, for the
// params struct, the reader and the variable of the handle of a key.
const (
	paramsWord = "params"
	readerWord = "of"
	keyWord    = "key"
)

// names are the Go identifiers that the generator declares for one spec,
// and the names of the group and the keys of the spec. An exported
// identifier is in Pascal case with each initialism in capitals, and an
// unexported one is in camel case, as the Go backend spells a declaration
// of the visibility. The lists are in the order of the params, the
// bindings and the roles of the spec.
type names struct {
	constant      string
	params        string
	reader        string
	familyVar     string
	idVar         string
	paramConsts   []string
	paramVars     []string
	bindingConsts []string
	bindingVars   []string
	roleConsts    []string
	group         string
	familyKey     string
	idKey         string
	paramKeys     []string
	bindingKeys   []string
}

// namesOf returns the Go identifiers and the key names of one spec. A key
// is the namespace, the name of the spec and a part, joined by dots, and
// the group is the namespace and the name. Only a contract has the key of
// its id and the variable of that key.
func namesOf(e entry) names {
	group := specfront.Namespace + partSep + e.name
	part := familyPart(e.form)
	n := names{
		constant:  naming.Pascal(e.name),
		params:    naming.Pascal(e.name + wordSep + paramsWord),
		reader:    naming.Pascal(e.name + wordSep + readerWord),
		familyVar: naming.Camel(e.name + wordSep + part + wordSep + keyWord),
		group:     group,
		familyKey: group + partSep + part,
	}
	if e.form == string(specfront.FormContract) {
		n.idVar = naming.Camel(e.name + wordSep + specfront.PartID + wordSep + keyWord)
		n.idKey = group + partSep + specfront.PartID
	}
	for _, p := range e.params {
		n.paramConsts = append(n.paramConsts, naming.Pascal(e.name+wordSep+p.key))
		n.paramVars = append(n.paramVars, naming.Camel(e.name+wordSep+p.key+wordSep+keyWord))
		n.paramKeys = append(n.paramKeys, group+partSep+p.key)
	}
	for _, b := range e.bindings {
		n.bindingConsts = append(n.bindingConsts, naming.Pascal(e.name+wordSep+b.name))
		n.bindingVars = append(n.bindingVars, naming.Camel(e.name+wordSep+b.name+wordSep+keyWord))
		n.bindingKeys = append(n.bindingKeys, group+partSep+b.name)
	}
	for _, r := range e.roles {
		n.roleConsts = append(n.roleConsts, naming.Pascal(e.name+wordSep+r.name))
	}
	return n
}

// all returns every Go identifier of the spec. A spec without an id has
// no variable of the id, so all leaves it out.
func (n names) all() []string {
	out := []string{n.constant, n.params, n.reader, n.familyVar}
	if n.idVar != "" {
		out = append(out, n.idVar)
	}
	out = append(out, n.paramConsts...)
	out = append(out, n.paramVars...)
	out = append(out, n.bindingConsts...)
	out = append(out, n.bindingVars...)
	return append(out, n.roleConsts...)
}

// familyPart returns the part of the family key of a form. A shape has the
// part shape, a mixin has the part mixin, and a contract has the part
// role, because the value of its family key is the role of the callable.
func familyPart(form string) string {
	switch form {
	case string(specfront.FormMixin):
		return specfront.PartMixin
	case string(specfront.FormContract):
		return specfront.PartRole
	default:
		return specfront.PartShape
	}
}
