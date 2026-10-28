// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"go.dokimi.dev/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/descriptor"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand of the units of the specs, whose carriers the
// fixtures write.
const brand = "fixture"

// parityDir is the directory of the fixtures that both frontends lower.
const parityDir = "testdata/parity"

// fixtureSources returns the sources of the parity fixtures by their proto
// paths, such as parity/proto3.proto.
func fixtureSources(tb testing.TB) map[string]string {
	tb.Helper()

	entries, err := os.ReadDir(parityDir)
	assert.NoError(tb, err, "the fixtures are on disk")
	out := map[string]string{}
	for _, e := range entries {
		src, err := os.ReadFile(filepath.Join(parityDir, e.Name()))
		assert.NoError(tb, err, e.Name()+" reads")
		out["parity/"+e.Name()] = string(src)
	}
	return out
}

// compiled compiles every parity fixture with protocompile at the standard
// source info, as buf does, and returns the descriptor of each file by its
// proto path.
func compiled(tb testing.TB) map[string]*descriptorpb.FileDescriptorProto {
	tb.Helper()

	return compiledFrom(tb, fixtureSources(tb))
}

// compiledFrom compiles sources, by their proto paths, as [compiled] does.
func compiledFrom(tb testing.TB, sources map[string]string) map[string]*descriptorpb.FileDescriptorProto {
	tb.Helper()

	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(sources),
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	files, err := c.Compile(tb.Context(), slices.Sorted(mapKeys(sources))...)
	assert.NoError(tb, err, "the fixtures compile")
	out := map[string]*descriptorpb.FileDescriptorProto{}
	for _, f := range files {
		// The descriptor that the compiler built keeps weak_dependency, which
		// protodesc drops, as protoc and buf keep it in a request.
		out[f.Path()] = f.(linker.Result).FileDescriptorProto()
		withImports(f, out)
	}
	return out
}

// withImports adds the descriptor of each file that f imports, at any
// depth, as a request of protoc lists every import in proto_file.
func withImports(f protoreflect.FileDescriptor, out map[string]*descriptorpb.FileDescriptorProto) {
	imports := f.Imports()
	for i := range imports.Len() {
		imported := imports.Get(i).FileDescriptor
		if _, held := out[imported.Path()]; held {
			continue
		}
		out[imported.Path()] = protodesc.ToFileDescriptorProto(imported)
		withImports(imported, out)
	}
}

// mapKeys returns the keys of a map of sources.
func mapKeys(m map[string]string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// registryOf returns the FileDescriptorSet of every descriptor, serialized.
func registryOf(tb testing.TB, fds map[string]*descriptorpb.FileDescriptorProto) []byte {
	tb.Helper()

	set := &descriptorpb.FileDescriptorSet{}
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range fds {
			if !yield(k) {
				return
			}
		}
	}) {
		set.File = append(set.File, fds[name])
	}
	b, err := proto.Marshal(set)
	assert.NoError(tb, err, "the registry encodes")
	return b
}

// viaDescriptor lowers the descriptor of one file through the descriptor
// frontend, as a unit of the workspace, and returns the builder and the
// sink of the unit.
func viaDescriptor(
	tb testing.TB, fd *descriptorpb.FileDescriptorProto, registry []byte,
) (*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	b, err := proto.Marshal(fd)
	assert.NoError(tb, err, "the descriptor encodes")
	tree := fstest.MapFS{fd.GetName(): {Data: b}}
	if registry != nil {
		tree[descriptor.Registry] = &fstest.MapFile{Data: registry}
	}
	f := descriptor.New()
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: fd.GetName(), Shared: []string{descriptor.Registry}}}, tree, plugin.DepthFull,
		f.Syntax(), brand, sink, f.Name(),
	)
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	return u.Graph(), sink
}

// parseErr parses the bytes of one file at name through the descriptor
// frontend, as a unit of the workspace, and returns the error of the parse.
// A nil b leaves the file out of the tree, and a nil registry leaves out
// [descriptor.Registry].
func parseErr(tb testing.TB, name string, b, registry []byte) error {
	tb.Helper()

	tree := fstest.MapFS{}
	if b != nil {
		tree[name] = &fstest.MapFile{Data: b}
	}
	if registry != nil {
		tree[descriptor.Registry] = &fstest.MapFile{Data: registry}
	}
	f := descriptor.New()
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: name, Shared: []string{descriptor.Registry}}}, tree, plugin.DepthFull,
		f.Syntax(), brand, diag.NewSink(), f.Name(),
	)
	return f.Parse(tb.Context(), u)
}

// viaBytes lowers the bytes of one file at name through the descriptor
// frontend, as a unit of the workspace or of a dependency, and returns the
// builder, the sink and the error of the parse.
func viaBytes(tb testing.TB, name string, b []byte, dependency bool) (*plugin.GraphBuilder, *diag.Sink, error) {
	tb.Helper()

	f := descriptor.New()
	sink := diag.NewSink()
	refs := []plugin.SourceRef{{Path: name, Shared: []string{descriptor.Registry}}}
	tree := fstest.MapFS{name: {Data: b}}
	u := plugin.NewSourceUnit(refs, tree, plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
	if dependency {
		u = plugin.NewDependencyUnit(refs, tree, f.Syntax(), brand, sink, f.Name())
	}
	err := f.Parse(tb.Context(), u)
	return u.Graph(), sink, err
}

// encoded returns the serialized bytes of a descriptor.
func encoded(tb testing.TB, fd *descriptorpb.FileDescriptorProto) []byte {
	tb.Helper()

	b, err := proto.Marshal(fd)
	assert.NoError(tb, err, "the descriptor encodes")
	return b
}

// stamped returns the dump lines of the stamps of one key.
func stamped(gb *plugin.GraphBuilder, key string) []string {
	var out []string
	for _, line := range dump(gb) {
		if strings.HasPrefix(line, "stamp ") && strings.Contains(line, " "+key+"=") {
			out = append(out, line)
		}
	}
	return out
}

// viaSource lowers the source of one file through the source frontend, and
// returns the builder and the sink of the unit.
func viaSource(tb testing.TB, name, src string) (*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	f := protofrontend.New()
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: name}}, fstest.MapFS{name: {Data: []byte(src)}}, plugin.DepthFull,
		f.Syntax(), brand, sink, f.Name(),
	)
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	return u.Graph(), sink
}

// dump returns a canonical text of what a builder contains: each package,
// file, import and declaration with its line, its documentation, its
// comment and its annotations, each type in its form with the last name of
// a named type, the attached directives, and the stamps. A stamp of an
// option has no spaces, and a stamp of ranges lists its entries in order,
// because the two frontends spell them from the source and from the
// descriptor.
func dump(gb *plugin.GraphBuilder) []string {
	d := &dumper{names: map[symbol.Symbol]string{}}
	for _, p := range gb.Packages() {
		d.line("package %q path=%q doc=%q", p.Name, p.Path, p.Doc)
		d.names[p] = "package " + strings.Join(p.Path, protobuf.NameSep)
		for _, f := range p.Files {
			d.names[f] = "file " + f.Path
			d.line("file %s line=%d doc=%q annotations=%v", f.Path, f.Pos.Line, f.Doc, f.Annotations)
			for _, imp := range f.Imports {
				d.line("  import %s line=%d doc=%q comment=%q", imp.Path, imp.Pos.Line, imp.Doc, imp.Comment)
			}
			for _, s := range f.Decls {
				d.decl("  ", "", s)
			}
		}
	}
	for _, a := range gb.Attachments() {
		d.line("attach %s %s %v line=%d", d.names[a.Subject], a.Raw.Name, a.Raw.Args, a.Raw.Pos.Line)
	}
	for _, r := range gb.StampRecords() {
		d.line("stamp %s %s=%s", d.names[r.Subject], r.Stamp.Key, stampValue(string(r.Stamp.Key), r.Stamp.Value))
	}
	return d.out
}

// dumper collects the lines of a dump and the names of the subjects.
type dumper struct {
	out   []string
	names map[symbol.Symbol]string
}

// line appends one formatted line.
func (d *dumper) line(format string, a ...any) { d.out = append(d.out, fmt.Sprintf(format, a...)) }

// decl appends the lines of one declaration and of its members under
// owner.
func (d *dumper) decl(indent, owner string, s symbol.Symbol) {
	switch v := s.(type) {
	case *node.Struct:
		d.names[v] = owner + v.Name
		d.line(
			"%sstruct %s line=%d doc=%q comment=%q annotations=%v",
			indent,
			v.Name,
			v.Pos.Line,
			v.Doc,
			v.Comment,
			v.Annotations,
		)
		for _, f := range v.Fields {
			d.field(indent+"  ", owner+v.Name+".", f)
		}
		for _, t := range v.Types {
			d.decl(indent+"  ", owner+v.Name+".", t)
		}
	case *node.Field:
		d.field(indent, owner, v)
	case *node.Sum:
		d.names[v] = owner + v.Name
		d.line("%ssum %s line=%d doc=%q comment=%q", indent, v.Name, v.Pos.Line, v.Doc, v.Comment)
		for _, variant := range v.Variants {
			for _, f := range variant.Fields {
				d.field(indent+"  ", owner+v.Name+".", f)
			}
		}
	case *node.Enum:
		d.names[v] = owner + v.Name
		d.line("%senum %s line=%d doc=%q comment=%q", indent, v.Name, v.Pos.Line, v.Doc, v.Comment)
		for _, variant := range v.Variants {
			d.names[variant] = owner + v.Name + "." + variant.Name
			d.line("%s  value %s=%s line=%d doc=%q comment=%q", indent, variant.Name, variant.Value,
				variant.Pos.Line, variant.Doc, variant.Comment)
		}
	case *node.Interface:
		d.names[v] = owner + v.Name
		d.line("%sinterface %s line=%d doc=%q comment=%q", indent, v.Name, v.Pos.Line, v.Doc, v.Comment)
		for _, m := range v.Methods {
			d.names[m] = owner + v.Name + "." + m.Name
			d.line("%s  rpc %s line=%d doc=%q in=%s out=%s async=%v", indent, m.Name, m.Pos.Line, m.Doc,
				typeOf(m.Params[0].Type), typeOf(m.Returns[0].Type), m.Async)
		}
	default:
		d.line("%s%T", indent, s)
	}
}

// field appends the line of one field.
func (d *dumper) field(indent, owner string, f *node.Field) {
	d.names[f] = owner + f.Name
	receives := ""
	if f.Receives != nil {
		receives = " receives=" + typeOf(f.Receives)
	}
	d.line("%sfield %s line=%d type=%s value=%q doc=%q comment=%q%s", indent, f.Name, f.Pos.Line, typeOf(f.Type),
		f.Value, f.Doc, f.Comment, receives)
}

// typeOf spells a type by its form and its arguments, with the spelling
// of a named type and the package of a well-known type.
func typeOf(t *node.TypeRef) string {
	if t == nil {
		return "-"
	}
	if t.Form != symbol.FormNamed {
		elems := make([]string, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = typeOf(e)
		}
		return fmt.Sprintf("form%d%q(%s)", t.Form, t.Spelling, strings.Join(elems, ","))
	}
	name := t.Spelling
	if t.Package != "" {
		return name + "@" + t.Package
	}
	return name
}

// stampValue spells the value of one stamp.
func stampValue(key string, v any) string {
	var entries []string
	switch value := v.(type) {
	case string:
		entries = []string{value}
	case []string:
		entries = value
	}
	switch key {
	case string(protobuf.OptionsKey), string(protobuf.FeaturesKey), string(protobuf.ExtensionsKey):
		for i, e := range entries {
			entries[i] = strings.ReplaceAll(e, " ", "")
		}
	case string(protobuf.ReservedKey):
		parts := make([]string, 0, len(entries))
		for _, e := range entries {
			parts = append(parts, strings.Split(e, ", ")...)
		}
		slices.Sort(parts)
		entries = parts
	}
	return strings.Join(entries, "|")
}

// codes returns the codes that a sink collected, in report order.
func codes(sink *diag.Sink) []diag.Code {
	out := []diag.Code{}
	for d := range sink.All() {
		out = append(out, d.Code)
	}
	return out
}
