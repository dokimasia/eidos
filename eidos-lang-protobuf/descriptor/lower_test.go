// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// The descriptor frontend lowers a descriptor into the graph that the
// source frontend lowers from the source of the same file.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		fds := compiled(t)
		registry := registryOf(t, fds)
		sources := fixtureSources(t)
		for _, name := range slices.Sorted(func(yield func(string) bool) {
			for k := range sources {
				if !yield(k) {
					return
				}
			}
		}) {
			t.Run("lowers "+name+" into the graph of the source frontend", func(t *testing.T) {
				t.Parallel()

				fromSource, sourceSink := viaSource(t, name, sources[name])
				fromDescriptor, descriptorSink := viaDescriptor(t, fds[name], registry)
				expect.Equal(t, dump(fromDescriptor), dump(fromSource), "the two frontends build one graph")
				assert.Equal(t, codes(descriptorSink), codes(sourceSink), "the two frontends report the same findings")
			})
		}

		t.Run("stamps local on each declaration that the visibility of edition 2024 makes local", func(t *testing.T) {
			t.Parallel()

			fd := &descriptorpb.FileDescriptorProto{
				Name:    new(visibleFile),
				Package: new("visible"),
				Syntax:  new("editions"),
				Edition: descriptorpb.Edition_EDITION_2024.Enum(),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: new("Outer"),
						NestedType: []*descriptorpb.DescriptorProto{
							{Name: new("Inner")},
							{
								Name:       new("Shared"),
								Visibility: descriptorpb.SymbolVisibility_VISIBILITY_EXPORT.Enum(),
							},
						},
					},
					{Name: new("Hidden"), Visibility: descriptorpb.SymbolVisibility_VISIBILITY_LOCAL.Enum()},
				},
				EnumType: []*descriptorpb.EnumDescriptorProto{
					{
						Name: new("Mode"),
						Value: []*descriptorpb.EnumValueDescriptorProto{
							{Name: new("MODE_UNSET"), Number: new(int32(0))},
						},
						Visibility: descriptorpb.SymbolVisibility_VISIBILITY_LOCAL.Enum(),
					},
				},
			}
			gb, _, err := viaBytes(t, visibleFile, encoded(t, fd), true)
			assert.NoError(t, err, "the parse succeeds")
			assert.Equal(t, stamped(gb, string(protobuf.LocalKey)), []string{
				"stamp Outer.Inner protobuf.local=local",
				"stamp Hidden protobuf.local=local",
				"stamp Mode protobuf.local=local",
			}, "a nested message, a local message and a local enum are local")
		})

		wholeMaps := []struct {
			name    string
			giveMap string
		}{
			{
				name:    "spells the value of a map whose type spans the whole map, as buf records it",
				giveMap: "map<int64, Nested>",
			},
			{name: "spells the value of a map written without a space after its comma", giveMap: "map<int64,Nested>"},
		}
		for _, tt := range wholeMaps {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				fd := proto.CloneOf(compiled(t)["parity/proto3.proto"])
				var at int32
				for i, f := range fd.GetMessageType()[0].GetField() {
					if f.GetName() == "by_id" {
						at = int32(i)
					}
				}
				for _, loc := range fd.GetSourceCodeInfo().GetLocation() {
					if slices.Equal(loc.GetPath(), []int32{4, 0, 2, at, 6}) {
						field := loc.GetSpan()
						loc.Span = []int32{field[0], 2, 2 + int32(len(tt.giveMap))}
					}
				}
				gb, _ := viaDescriptor(t, fd, nil)
				expect.That(t, strings.Join(dump(gb), "\n")).
					Contains(`field by_id line=19 type=form4"map<int64, Nested>"(int64,Nested)`,
						"the value is spelled as written")
			})
		}

		t.Run("stamps an option import among the imports of the file", func(t *testing.T) {
			t.Parallel()

			fd := &descriptorpb.FileDescriptorProto{
				Name:             new(visibleFile),
				Package:          new("visible"),
				Syntax:           new("editions"),
				Edition:          descriptorpb.Edition_EDITION_2024.Enum(),
				Dependency:       []string{"parity/common.proto"},
				OptionDependency: []string{"parity/custom.proto"},
			}
			gb, _, err := viaBytes(t, visibleFile, encoded(t, fd), true)
			assert.NoError(t, err, "the parse succeeds")
			assert.Equal(t, stamped(gb, string(protobuf.ImportKey)), []string{
				"stamp file parity/visible.proto protobuf.import=parity/common.proto|option parity/custom.proto",
			}, "the option import follows the imports of types")
		})

		t.Run("lowers a dependency without source info", func(t *testing.T) {
			t.Parallel()

			fd := compiled(t)["parity/proto2.proto"]
			fd.SourceCodeInfo = nil
			gb, sink, err := viaBytes(t, fd.GetName(), encoded(t, fd), true)
			assert.NoError(t, err, "the parse succeeds")
			expect.Empty(t, codes(sink), "a dependency reports no loss of comments")
			expect.That(t, strings.Join(dump(gb), "\n")).
				Contains("field extra_note", "the extension loads without the location of its block")
		})
	})
}

// visibleFile is the proto path of the visibility case.
const visibleFile = "parity/visible.proto"
