// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// The schema of the option cases: an enum option, a list option, a
// message option and a feature of the file.
const (
	optionsFile = "parity/options.proto"
	optionsSrc  = "edition = \"2023\";\n\npackage options;\n\nimport \"google/protobuf/descriptor.proto\";\n\n" +
		"option optimize_for = SPEED;\noption features.field_presence = IMPLICIT;\n" +
		"option (options.labels) = \"a\";\noption (options.labels) = \"b\";\n" +
		"option (options.config) = {name: \"c\"};\n\n" +
		"message Config {\n  string name = 1;\n}\n\n" +
		"extend google.protobuf.FileOptions {\n  repeated string labels = 50001;\n  Config config = 50002;\n}\n"
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

		t.Run("stamps the features of a file one entry for each feature", func(t *testing.T) {
			t.Parallel()

			fromDescriptor, _ := viaDescriptor(t, fds[optionsFile], registry)
			assert.Equal(t, stamped(fromDescriptor, string(protobuf.FeaturesKey)),
				[]string{"stamp file parity/options.proto protobuf.features=features.field_presence=IMPLICIT"},
				"the feature is stamped by its name")
		})
	})
}
