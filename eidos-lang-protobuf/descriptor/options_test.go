// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// The schema of the option cases: an enum option, a list option, a
// message option and a feature of the file. The message literal is written
// in the spelling of the descriptor frontend, with a list, a nested message
// and a map.
const (
	optionsFile = "parity/options.proto"
	optionsSrc  = "edition = \"2023\";\n\npackage options;\n\nimport \"google/protobuf/descriptor.proto\";\n\n" +
		"option optimize_for = SPEED;\noption features.field_presence = IMPLICIT;\n" +
		"option (options.labels) = \"a\";\noption (options.labels) = \"b\";\n" +
		"option (options.config) = " + configLiteral + ";\n\n" +
		"message Config {\n  string name = 1;\n  repeated string tags = 2;\n  Inner inner = 3;\n" +
		"  map<string, int32> limits = 4;\n  map<int32, int32> by_int = 5;\n  map<uint64, int32> by_uint = 6;\n" +
		"  map<bool, int32> by_flag = 7;\n}\n\n" +
		"message Inner {\n  int32 size = 1;\n  extensions 100 to 199;\n}\n\n" +
		"extend Inner {\n  int32 weight = 100;\n}\n\n" +
		"extend google.protobuf.FileOptions {\n  repeated string labels = 50001;\n  Config config = 50002;\n}\n"
	// configLiteral is the message option of the schema, in the order of
	// the field numbers and with each map in key order.
	configLiteral = `{name: "c", tags: ["x", "y"], inner: {size: 3, [options.weight]: 4}, ` +
		`limits: [{key: "a", value: 1}, {key: "b", value: 2}], by_int: [{key: 2, value: 1}, {key: 10, value: 2}], ` +
		`by_uint: [{key: 2, value: 1}, {key: 10, value: 2}], by_flag: [{key: false, value: 1}, {key: true, value: 2}]}`
)

func TestOptions(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		fds := compiledFrom(t, map[string]string{optionsFile: optionsSrc})
		registry := registryOf(t, fds)

		t.Run("stamps the options of a file in the spelling of the source frontend", func(t *testing.T) {
			t.Parallel()

			fromSource, _ := viaSource(t, optionsFile, optionsSrc)
			fromDescriptor, _ := viaDescriptor(t, fds[optionsFile], registry)
			assert.Equal(t, stamped(fromDescriptor, string(protobuf.OptionsKey)),
				stamped(fromSource, string(protobuf.OptionsKey)), "the two frontends stamp the same options")
		})

		t.Run("spells a message option in one form whatever the build of the binary", func(t *testing.T) {
			t.Parallel()

			fromDescriptor, _ := viaDescriptor(t, fds[optionsFile], registry)
			var entries []string
			for _, r := range fromDescriptor.StampRecords() {
				if r.Stamp.Key == protobuf.OptionsKey {
					entries = r.Stamp.Value.([]string)
				}
			}
			assert.Contains(t, entries, "(options.config)="+configLiteral, "the literal has its spaces and its order")
		})

		t.Run("stamps the features of a file one entry for each feature", func(t *testing.T) {
			t.Parallel()

			fromDescriptor, _ := viaDescriptor(t, fds[optionsFile], registry)
			assert.Equal(t, stamped(fromDescriptor, string(protobuf.FeaturesKey)),
				[]string{"stamp file parity/options.proto protobuf.features=features.field_presence=IMPLICIT"},
				"the feature is stamped by its name")
		})
	})
}
