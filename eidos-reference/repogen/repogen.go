// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package repogen

import (
	"embed"
	"fmt"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "repogen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that repogen provides. A plugin that
// extends a repository or its implementation requires it.
const Capability plugin.Capability = "repository"

// RepoDirective is the directive that puts a struct under repogen.
const RepoDirective directive.Name = "repo"

// KeyRepository is the key of the fact that repogen stamps on each struct
// under the directive repo. A plugin that extends a repository selects its
// subjects through the fact, with a named handle.
const KeyRepository meta.KeyName = "repogen.repository"

// Word is the word of the family of the repositories.
const Word = "repo"

// The words that the names of a repository and of its implementation join
// onto the name of the struct.
const (
	InterfaceWord = "repository"
	StructWord    = "repo"
)

// The template of the body of a method of an implementation.
const bodyTemplate = "repo.tmpl"

// The context, the key and the value that the methods take, the types of
// Go they have, and the marks of a pointer and of a list.
const (
	ctxParam     = "ctx"
	keyParam     = "id"
	valueParam   = "v"
	contextType  = "context.Context"
	contextPkg   = "context"
	stringType   = "string"
	errorType    = "error"
	pointerMark  = "*"
	listMark     = "[]"
	interfaceDoc = "%s stores the values of %s."
	zeroDoc      = "%s implements %s with zero values."
)

// result is what a method of a repository returns beside its error.
type result uint8

// The results of a method beside its error: nothing, one value, or a list
// of values.
const (
	resultNone  result = 1
	resultValue result = 2
	resultList  result = 3
)

// repoMethod is one method of a repository: its name, the format of its
// documentation in the repository, its parameter after the context, and its
// result beside the error. The format takes the name of the struct.
type repoMethod struct {
	name   string
	doc    string
	param  string
	result result
}

// methods are the methods of a repository, in the order of the
// declarations.
var methods = []repoMethod{
	{name: "Get", doc: "Get returns the %s with the identifier id.", param: keyParam, result: resultValue},
	{name: "List", doc: "List returns every %s.", result: resultList},
	{name: "Put", doc: "Put stores the %s v.", param: valueParam, result: resultNone},
	{name: "Delete", doc: "Delete removes the %s with the identifier id.", param: keyParam, result: resultNone},
}

// schema is the directive of repogen.
var schema = directive.Schema{
	Plugin: string(ID),
	Name:   RepoDirective,
	Doc:    "generates a repository of the struct and an implementation of it with zero values",
}

// templates are the templates of repogen, below the directory of each
// target.
//
//go:embed templates
var templates embed.FS

// conclusion is the state of one instance of repogen: the handle of its
// key, which the registration sets and the annotator stamps through.
//
// # Concurrency
//
// The composition registers the keys before the first phase call, and the
// handlers read the handle afterwards, so the workers of a phase call read
// it concurrently.
type conclusion struct {
	repository meta.Key[bool]
}

// New returns repogen. Its annotator stamps [KeyRepository] on each struct
// under [RepoDirective]. Its generator writes, for each struct with the
// fact, the interface <T>Repository with the methods Get, List, Put and
// Delete, and the struct <T>Repo, whose methods implement them on a pointer
// receiver. The template repo.tmpl writes the body of each method of the
// struct: the slots, and the return of the zero values of the results. The
// plugin serves the target golang alone. Each call returns a new plugin,
// because a plugin instance belongs to one workspace.
func New() sdk.Dual {
	c := &conclusion{}
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Output(plugin.Output{Per: plugin.PerSource, Word: Word}).
		For(golang.Target, sdk.Templates(tree.Of(templates, golang.Target))).
		Keys(c.register).
		Handle(
			sdk.Directive(schema, sdk.OnStruct(c.conclude)),
			sdk.Where(sdk.HasKey(meta.Named[bool](KeyRepository)), sdk.OnStruct(generate)),
		).
		Dual()
}

// register claims the namespace of repogen and registers [KeyRepository].
//
// Error modes: the error of the claim, such as a namespace that another
// registrant claimed, and the error of the registration.
func (c *conclusion) register(r *meta.Registry) error {
	if err := r.ClaimNamespace(KeyRepository.Namespace()); err != nil {
		return err
	}
	var err error
	c.repository, err = meta.Register[bool](r, meta.KeySpec{
		Name:  KeyRepository,
		Kinds: []symbol.Kind{symbol.KindStruct},
		Doc:   "marks a struct that repogen writes a repository of",
	})
	return err
}

// conclude stamps [KeyRepository] on a struct under the directive.
func (c *conclusion) conclude(_ *sdk.StructMatch, st *sdk.Stamper) error {
	sdk.Stamp(st, c.repository, true)
	return nil
}

// generate writes the repository of one struct and its implementation into
// the family of the repositories, beside the source of the struct.
func generate(m *sdk.StructMatch, e *sdk.Emitter) error {
	s := m.Struct
	iface := e.JoinName(InterfaceWord, s.Name)
	impl := e.JoinName(StructWord, s.Name)
	repo := &emit.Interface{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(interfaceDoc, iface, s.Name)},
		Name:       iface,
		Visibility: symbol.VisibilityPublic,
	}
	zero := &emit.Struct{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(zeroDoc, impl, iface)},
		Name:       impl,
		Visibility: symbol.VisibilityPublic,
	}
	for _, rm := range methods {
		params, returns := signature(s, rm)
		repo.Methods.Append(&emit.Method{
			Origin:     s.ID,
			Doc:        []string{fmt.Sprintf(rm.doc, s.Name)},
			Name:       rm.name,
			Visibility: symbol.VisibilityPublic,
			Params:     emit.SlotOf(params...),
			Returns:    emit.SlotOf(returns...),
		})
		params, returns = signature(s, rm)
		method := golang.PointerReceiver(&emit.Method{
			Origin:     s.ID,
			Doc:        []string{fmt.Sprintf(zeroDoc, rm.name, iface)},
			Name:       rm.name,
			Visibility: symbol.VisibilityPublic,
			Receives:   &emit.TypeRef{Spelling: impl},
			Params:     emit.SlotOf(params...),
			Returns:    emit.SlotOf(returns...),
		})
		method.Body.Ref = e.Ref(bodyTemplate, nil)
		zero.Methods.Append(method)
	}
	e.File().Append(repo, zero)
	return nil
}

// signature returns the parameters and the results of the method rm of a
// repository of the struct s: the context, the key or the value of rm, and
// the value, the list or nothing beside the error. Each call returns lists
// and references of its own, so a declaration of the repository and one of
// its implementation share none.
func signature(s *node.Struct, rm repoMethod) ([]*emit.Param, []*emit.Return) {
	value := &emit.TypeRef{
		Spelling: pointerMark + s.Name, Form: symbol.FormOptional,
		Elems: []*emit.TypeRef{{Spelling: s.Name, Target: s.ID}},
	}
	params := []*emit.Param{{Name: ctxParam, Type: &emit.TypeRef{Spelling: contextType, Package: contextPkg}}}
	switch rm.param {
	case keyParam:
		params = append(params, &emit.Param{Name: keyParam, Type: &emit.TypeRef{Spelling: stringType}})
	case valueParam:
		params = append(params, &emit.Param{Name: valueParam, Type: value})
	}
	errResult := &emit.Return{Type: &emit.TypeRef{Spelling: errorType}}
	switch rm.result {
	case resultValue:
		return params, []*emit.Return{{Type: value}, errResult}
	case resultList:
		list := &emit.TypeRef{Spelling: listMark + value.Spelling, Form: symbol.FormList, Elems: []*emit.TypeRef{value}}
		return params, []*emit.Return{{Type: list}, errResult}
	default:
		return params, []*emit.Return{errResult}
	}
}
