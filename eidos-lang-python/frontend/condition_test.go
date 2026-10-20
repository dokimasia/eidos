// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// TestCondition pins the branches that the checks of conditions select for
// the version 3.15 and the platform linux of the empty options.
func TestCondition(t *testing.T) {
	t.Parallel()

	t.Run("branches", func(t *testing.T) {
		t.Parallel()

		t.Run("loads the branch of a check that is true and no later branch", func(t *testing.T) {
			t.Parallel()

			names := branchNames(t, "import sys\nif sys.version_info >= (3, 12):\n    a = 1\n"+
				"elif sys.version_info >= (3, 10):\n    b = 1\nelse:\n    c = 1\n")
			assert.Equal(t, names, []string{"a"}, "the first true check decides")
		})

		t.Run("loads the else of checks that are false", func(t *testing.T) {
			t.Parallel()

			names := branchNames(t, "import sys\nif sys.platform == \"win32\":\n    a = 1\nelse:\n    b = 1\n")
			assert.Equal(t, names, []string{"b"}, "the platform is linux")
		})

		t.Run("loads every branch of a condition that the frontend does not evaluate", func(t *testing.T) {
			t.Parallel()

			names := branchNames(t, "if DEBUG:\n    a = 1\nelse:\n    b = 1\n")
			assert.Equal(t, names, []string{"a", "b"}, "a type checker reads both branches")
		})

		t.Run("stamps the file with each check that excluded a branch once", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import sys\nif sys.version_info < (3, 12):\n    a = 1\n"+
				"if sys.version_info < (3, 12):\n    b = 1\n")
			assert.Equal(t, stampsOn(gb, fileOf(t, gb), python.ConditionKey), []any{"sys.version_info<(3,12)"},
				"the check as written, once")
		})

		t.Run("reads the options of the frontend", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(&frontend.Options{Version: "3.11", Platform: "win32"})
			tree := fstest.MapFS{appFile: {Data: []byte("import sys\nif sys.version_info >= (3, 12):\n    a = 1\n" +
				"if sys.platform == \"win32\":\n    b = 1\n")}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile}}, tree, plugin.DepthFull, f.Syntax(), brand,
				diag.NewSink(), f.Name())
			assert.NoError(t, f.Parse(t.Context(), u), "the module parses")
			assert.Equal(t, declNames(u.Graph().Packages()[0].Files[0]), []string{"b"},
				"Python 3.11 on Windows reads the second branch alone")
		})
	})

	t.Run("check", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want []string
		}{
			{
				name: "reads TYPE_CHECKING as true",
				give: "if TYPE_CHECKING:\n    a = 1\nelse:\n    b = 1\n",
				want: []string{"a"},
			},
			{
				name: "reads the TYPE_CHECKING of typing as true",
				give: "import typing\nif typing.TYPE_CHECKING:\n    a = 1\nelse:\n    b = 1\n", want: []string{"a"},
			},
			{name: "reads not of a true check as false", give: "if not TYPE_CHECKING:\n    a = 1\n", want: []string{}},
			{
				name: "reads not of a false check as true",
				give: "import sys\nif not sys.platform == \"win32\":\n    a = 1\nelse:\n    b = 1\n",
				want: []string{"a"},
			},
			{
				name: "reads not of an unknown condition as unknown", give: "if not DEBUG:\n    a = 1\n",
				want: []string{"a"},
			},
			{
				name: "reads and of a false check as false",
				give: "import sys\nif DEBUG and sys.platform == \"darwin\":\n    a = 1\n", want: []string{},
			},
			{
				name: "reads and of true checks as true",
				give: "import sys\nif TYPE_CHECKING and sys.platform == \"linux\":\n    a = 1\nelse:\n    b = 1\n",
				want: []string{"a"},
			},
			{
				name: "reads or of a true check as true",
				give: "if DEBUG or TYPE_CHECKING:\n    a = 1\nelse:\n    b = 1\n", want: []string{"a"},
			},
			{
				name: "reads or of false checks as false",
				give: "import sys\nif sys.platform == \"a\" or sys.platform == \"b\":\n    a = 1\nelse:\n    b = 1\n",
				want: []string{"b"},
			},
			{
				name: "reads and of a true check and an unknown condition as unknown",
				give: "if TYPE_CHECKING and DEBUG:\n    a = 1\nelse:\n    b = 1\n", want: []string{"a", "b"},
			},
			{
				name: "reads a parenthesized check",
				give: "if (TYPE_CHECKING):\n    a = 1\nelse:\n    b = 1\n",
				want: []string{"a"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, branchNames(t, tt.give), tt.want, "the branches that the check selects")
			})
		}
	})

	t.Run("compare", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "compares the version with a tuple", give: "sys.version_info >= (3, 15)", want: true},
			{name: "compares the version with a later version", give: "sys.version_info < (3, 15)", want: false},
			{name: "compares the major version with an integer", give: "sys.version_info[0] == 3", want: true},
			{name: "compares the minor version with an integer", give: "sys.version_info[1] > 14", want: true},
			{
				name: "compares a slice of the version with a tuple",
				give: "sys.version_info[:2] != (3, 15)",
				want: false,
			},
			{
				name: "compares the version with a tuple of the major version",
				give: "sys.version_info <= (3,)",
				want: true,
			},
			{name: "compares the platform with ==", give: "sys.platform == \"linux\"", want: true},
			{name: "compares the platform with !=", give: "sys.platform != \"linux\"", want: false},
			{name: "tests the prefix of the platform", give: "sys.platform.startswith(\"lin\")", want: true},
			{name: "tests another prefix of the platform", give: "sys.platform.startswith(\"win\")", want: false},
			{name: "reads the version that from sys import binds", give: "version_info > (3, 0)", want: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				src := "import sys\nfrom sys import version_info\nif " + tt.give + ":\n    a = 1\nelse:\n    b = 1\n"
				names := branchNames(t, src)
				want := []string{"b"}
				if tt.want {
					want = []string{"a"}
				}
				assert.Equal(t, names, want, "the branch of the value of the check")
			})
		}

		unknowns := []struct {
			name string
			give string
		}{
			{name: "reads a micro version that would decide as unknown", give: "sys.version_info >= (3, 15, 1)"},
			{name: "reads a chain of comparisons as unknown", give: "(3, 0) < sys.version_info < (4, 0)"},
			{name: "reads a comparison of the platform by an order as unknown", give: "sys.platform < \"m\""},
			{name: "reads a comparison of the platform with a name as unknown", give: "sys.platform == name"},
			{name: "reads a version compared with a name as unknown", give: "sys.version_info >= minimum"},
			{name: "reads a version compared by is as unknown", give: "sys.version_info is v"},
			{name: "reads a version compared by is with a tuple as unknown", give: "sys.version_info is (3, 12)"},
			{name: "reads a hexadecimal element of a version as unknown", give: "sys.version_info >= (3, 0x10)"},
			{name: "reads another element of the version as unknown", give: "sys.version_info[2] == 0"},
			{name: "reads a comparison of another member of sys as unknown", give: "sys.maxsize > 2"},
			{name: "reads a call of another method of the platform as unknown", give: "sys.platform.endswith(\"x\")"},
			{name: "reads a prefix that is no string as unknown", give: "sys.platform.startswith(prefix)"},
		}
		for _, tt := range unknowns {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				names := branchNames(t, "import sys\nif "+tt.give+":\n    a = 1\nelse:\n    b = 1\n")
				expect.Equal(t, names, []string{"a", "b"}, "both branches load")
			})
		}
	})
}

// branchNames parses one source at appFile and returns the names of the
// declarations of its file.
func branchNames(tb testing.TB, src string) []string {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	return declNames(fileOf(tb, gb))
}
