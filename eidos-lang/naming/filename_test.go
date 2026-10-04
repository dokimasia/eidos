// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/naming"
)

// The parts a filename fixture builds from: a unit's file key, a family
// word, a tag and an extension.
const (
	unitKey    = "svc/store/row.go"
	familyWord = "stub"
	testTag    = "test"
	rustExt    = ".rs"
)

// The allocations of the filename helpers.
const (
	// partsAllocs is the list of a filename's parts.
	partsAllocs = 1
	// snakeFilenameAllocs is a filename whose joined parts are snake
	// case already: the list of parts, their join, and the name with its
	// extension.
	snakeFilenameAllocs = 3
	// convertedFilenameAllocs is a filename whose joined parts convert:
	// the list of parts, their join, the snake-cased join, and the name
	// with its extension.
	convertedFilenameAllocs = 4
)

// The filename parts and the snake-cased shape are pinned once for
// every target that builds a filename from them.
func TestFilename(t *testing.T) {
	t.Parallel()

	t.Run("FilenameParts", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			key, word, tag string
			want           []string
		}{
			{
				name: "orders the key's stem before the word before the tag",
				key:  unitKey, word: familyWord, tag: testTag, want: []string{"row", familyWord, testTag},
			},
			{name: "leaves out an empty part", word: "HTTPClient", want: []string{"HTTPClient"}},
			{name: "returns no part for empty inputs", want: []string{}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, naming.FilenameParts(tt.key, tt.word, tt.tag), tt.want, "the filename's parts")
			})
		}
	})

	t.Run("SnakeFilename", func(t *testing.T) {
		t.Parallel()

		t.Run("joins the parts with underscores before the extension", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, naming.SnakeFilename(unitKey, familyWord, "", rustExt), "row_stub.rs",
				"the stem drops its extension and joins the word")
		})

		t.Run("snake-cases the joined parts", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, naming.SnakeFilename("", "HTTPClient", testTag, ".go"), "http_client_test.go",
				"a plan unit's word and tag, snake-cased as one")
		})
	})
}

// The filename helpers allocate their parts and their names in the
// ordinary run, which runs no benchmark.
func TestFilenameAllocs(t *testing.T) {
	var (
		parts []string
		name  string
	)
	assert.MaxAllocs(t, func() { parts = naming.FilenameParts(unitKey, familyWord, testTag) }, partsAllocs,
		"FilenameParts allocates the list")
	assert.Length(t, parts, 3, "FilenameParts returns the three parts")
	assert.MaxAllocs(t, func() { name = naming.SnakeFilename(unitKey, familyWord, "", rustExt) }, snakeFilenameAllocs,
		"SnakeFilename allocates the parts, their join and the name")
	assert.Equal(t, name, "row_stub.rs", "SnakeFilename joins the parts")
	assert.MaxAllocs(t, func() { name = naming.SnakeFilename("", "HTTPClient", testTag, ".go") },
		convertedFilenameAllocs, "SnakeFilename allocates the conversion of a join in another case")
	assert.Equal(t, name, "http_client_test.go", "SnakeFilename converts the join")
}

// BenchmarkFilename measures the parts and the snake-cased name a
// backend builds once per unit.
func BenchmarkFilename(b *testing.B) {
	b.Run("FilenameParts", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(partsAllocs)
		defer c.End()
		var parts []string
		for c.Loop() {
			parts = naming.FilenameParts(unitKey, familyWord, testTag)
		}
		assert.Length(b, parts, 3, "FilenameParts returns the three parts")
	})

	b.Run("SnakeFilename", func(b *testing.B) {
		b.Run("a join already in snake case", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(snakeFilenameAllocs)
			defer c.End()
			var name string
			for c.Loop() {
				name = naming.SnakeFilename(unitKey, familyWord, "", rustExt)
			}
			assert.Equal(b, name, "row_stub.rs", "SnakeFilename joins the parts")
		})

		b.Run("a join in another case", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(convertedFilenameAllocs)
			defer c.End()
			var name string
			for c.Loop() {
				name = naming.SnakeFilename("", "HTTPClient", testTag, ".go")
			}
			assert.Equal(b, name, "http_client_test.go", "SnakeFilename converts the join")
		})
	})
}
