// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc_test

import (
	"bytes"
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"go.dokimi.dev/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/protoc"
)

// The schema of the specs: a file to generate that imports a file that
// the request does not generate.
const (
	rowsFile   = "acme/v1/rows.proto"
	sharedFile = "acme/v1/shared.proto"
	rowsSrc    = "syntax = \"proto3\";\n\npackage acme.v1;\n\nimport \"acme/v1/shared.proto\";\n\n" +
		"// Row is one row.\nmessage Row {\n  Shared shared = 1;\n}\n"
	sharedSrc = "syntax = \"proto3\";\n\npackage acme.v1;\n\n// Shared is imported.\nmessage Shared {\n  string id = 1;\n}\n"
	// rowsList is the file that the lister writes beside the file to
	// generate.
	rowsList = "acme/v1/rows_list.txt"
)

// The composition of the specs.
const (
	brand              = "acme"
	listerID plugin.ID = "lister"
	target             = "fixture"
)

// listerOptions are the options of the lister: the suffix of the name of
// each struct that it writes.
type listerOptions struct {
	Suffix string `opt:"suffix" doc:"the suffix of the name of each struct"`
}

// compose returns the composition of the specs: the brand, the rules of
// protobuf, and one plan whose lister writes one struct for each message
// of a file to generate into a file beside it.
func compose() *workspace.Builder {
	opts := &listerOptions{Suffix: "List"}
	lister := eidos.NewPlugin(listerID).
		Options(opts).
		Output(plugin.Output{Per: plugin.PerSource, Word: "list"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.File().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: m.Struct.Name + opts.Suffix})
			return nil
		})).Generator()
	return workspace.New().
		Brand(brand).
		Targets(target).
		Rules(protorules.New()).
		Plans(workspace.Plan{Name: "lists", Generators: []plugin.Generator{lister}, Backend: printer()})
}

// goCompose returns a composition whose plan writes a Go struct with the
// fields of each message of a file to generate, through the Go backend.
func goCompose() *workspace.Builder {
	models := eidos.NewPlugin("models").
		Output(plugin.Output{Per: plugin.PerSource, Word: "models"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			fields, spelled := e.Fields(m.Struct.Fields, nil)
			if !spelled {
				return nil
			}
			e.File().
				Append(&emit.Struct{Origin: m.Struct.Identity(), Name: m.Struct.Name, Fields: emit.SlotOf(fields...)})
			return nil
		})).Generator()
	return workspace.New().
		Brand(brand).
		Targets(golang.Target).
		Rules(protorules.New()).
		Plans(workspace.Plan{
			Name: "go", Generators: []plugin.Generator{models}, Backend: gobackend.New(),
			Layout: layout.Config{ImportBase: "example.com/app/gen"},
		})
}

// printer returns a kit backend of the target that writes each struct as
// one line, into a file named after the proto file and the word of the
// family.
func printer() plugin.Backend {
	return backend.New("printer", target, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string {
			return strings.TrimSuffix(path.Base(u.Key), ".proto") + "_" + u.Word + ".txt"
		}).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the printer spells no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: allFacts()}).
		Build()
}

// allFacts returns a verdict of Renders for every fact.
func allFacts() map[symbol.Fact]render.Verdict {
	out := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		out[f] = render.Renders
	}
	return out
}

// request returns the request of the schema of the specs, which generates
// the rows file, with parameter as its parameter.
func request(tb testing.TB, parameter string) *pluginpb.CodeGeneratorRequest {
	tb.Helper()

	return requestOf(tb, rowsSrc, parameter)
}

// requestOf returns the request of the schema of the specs with src as the
// source of the rows file.
func requestOf(tb testing.TB, src, parameter string) *pluginpb.CodeGeneratorRequest {
	tb.Helper()

	c := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(map[string]string{rowsFile: src, sharedFile: sharedSrc}),
		},
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	files, err := c.Compile(tb.Context(), rowsFile)
	assert.NoError(tb, err, "the schema compiles")
	rows := protodesc.ToFileDescriptorProto(files[0].(linker.Result))
	shared := protodesc.ToFileDescriptorProto(files[0].Imports().Get(0).FileDescriptor)
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{rowsFile},
		ProtoFile:      []*descriptorpb.FileDescriptorProto{shared, rows},
	}
	if parameter != "" {
		req.Parameter = new(parameter)
	}
	return req
}

// runErr runs the composition of the specs over a request, and returns the
// error of the run.
func runErr(tb testing.TB, req *pluginpb.CodeGeneratorRequest) error {
	tb.Helper()

	in, err := proto.Marshal(req)
	assert.NoError(tb, err, "the request encodes")
	var out, log bytes.Buffer
	return protoc.Run(tb.Context(), bytes.NewReader(in), &out, &log, compose)
}

// answered runs the composition of compose over a request, and returns the
// response and what the run logged.
func answered(
	tb testing.TB,
	req *pluginpb.CodeGeneratorRequest,
	compose protoc.Compose,
) (*pluginpb.CodeGeneratorResponse, string) {
	tb.Helper()

	in, err := proto.Marshal(req)
	assert.NoError(tb, err, "the request encodes")
	var out, log bytes.Buffer
	assert.NoError(tb, protoc.Run(tb.Context(), bytes.NewReader(in), &out, &log, compose), "the run answers")
	resp := &pluginpb.CodeGeneratorResponse{}
	assert.NoError(tb, proto.Unmarshal(out.Bytes(), resp), "the response decodes")
	return resp, log.String()
}
