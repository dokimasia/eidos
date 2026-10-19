// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf/backend"
)

// checkedFile is a file of proto3, which the allocation test and the
// benchmark check.
const checkedFile = "syntax = \"proto3\";\n\npackage acme.svc;\n\n" +
	"// Row is one record.\nmessage Row {\n  string name = 1; // the key\n}\n"

// checkedAllocs is the check of [checkedFile]: the copy of the file, and the
// 50 allocations of its parse inside protocompile.
const checkedAllocs = 51

// TestCheck pins the finalising step of a written file: a file that
// parses passes as it is, and a file that does not parse is refused at the
// line of its first error.
func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
		}{
			{name: "returns a file of proto3 as it is", give: checkedFile},
			{
				name: "returns a file of edition 2026 as it is",
				give: "edition = \"2026\";\n\nmessage Row {\n  int32 id = 1;\n}\n",
			},
			{
				name: "returns a file outside the layout of buf format as it is",
				give: "syntax = \"proto3\";\nmessage   Row {string name=1;}\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Check([]byte(tt.give))
				assert.NoError(t, err, "the file parses")
				assert.Equal(t, string(got), tt.give, "the check keeps the file")
			})
		}

		refusals := []struct {
			name string
			give string
			want string
		}{
			{
				name: "returns an error for a file that does not parse",
				give: "syntax = \"proto3\";\nmessage Row {\n",
				want: "protobuf: the written file does not parse at line 2:",
			},
			{
				name: "returns an error for a label that the version of the file does not have",
				give: "syntax = \"proto3\";\nmessage Row {\n  required int32 id = 1;\n}\n",
				want: "protobuf: the written file does not parse at line 3:",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Check([]byte(tt.give))
				assert.HasError(t, err, "the check refuses the file")
				assert.HasPrefix(t, err.Error(), tt.want, "the error states the line of the first error")
				assert.Nil(t, got, "a refusal returns no file")
			})
		}
	})
}

// Check allocates the copy and the parse of a file. The ordinary run,
// which runs no benchmark, checks that ceiling here.
func TestCheckAllocs(t *testing.T) {
	src := []byte(checkedFile)
	var (
		got []byte
		err error
	)
	assert.MaxAllocs(t, func() { got, err = backend.Check(src) }, checkedAllocs,
		"Check allocates the copy and the parse of the file")
	assert.NoError(t, err, "Check passes the file")
	assert.Equal(t, string(got), checkedFile, "Check keeps the file")
}

// BenchmarkCheck measures the finalising step that the render runs on
// every written file.
func BenchmarkCheck(b *testing.B) {
	b.Run("Check", func(b *testing.B) {
		src := []byte(checkedFile)
		c := bench.Start(b).MaxAllocs(checkedAllocs)
		defer c.End()
		var (
			got []byte
			err error
		)
		for c.Loop() {
			got, err = backend.Check(src)
		}
		assert.NoError(b, err, "Check passes the file")
		assert.Equal(b, string(got), checkedFile, "Check keeps the file")
	})
}
