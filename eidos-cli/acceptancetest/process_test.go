// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package acceptancetest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest"
)

// probeEnv makes the test binary write its environment to standard output
// as a JSON array in place of its tests, when it has the value probeValue.
// A case then runs the test binary through Exec to read the environment
// that Exec passes.
const (
	probeEnv   = "ACCEPTANCETEST_PROBE"
	probeValue = "1"
)

// replacedVar is the variable of the test process that a case replaces
// through the env of Exec, and replacedValue is its new value.
const (
	replacedVar   = "HOME"
	replacedValue = "/acceptancetest/home"
)

// The command that a case runs, the first line of its output, and the name
// of a binary that does not exist.
const (
	planCommand = "plan"
	planLine    = "frontend fakefront 1\n"
	absentName  = "absent"
)

// TestMain writes the environment of the process in place of the tests
// when probeEnv has the value probeValue, and runs the tests otherwise.
func TestMain(m *testing.M) {
	if os.Getenv(probeEnv) == probeValue {
		env, _ := json.Marshal(os.Environ())
		fmt.Println(string(env))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// Build compiles a main package into a binary of the test, and Exec runs a
// binary under the environment of the test process.
func TestProcess(t *testing.T) {
	t.Parallel()

	bin := acceptancetest.Build(t, hostMain)

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the path of the binary of the main package", func(t *testing.T) {
			t.Parallel()

			got := acceptancetest.Exec(t, bin, t.TempDir(), nil, crashName)
			assert.Equal(t, got.Status, panicStatus, "the binary runs the crash command of the host")
		})
	})

	t.Run("Exec", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the status and the output of the process", func(t *testing.T) {
			t.Parallel()

			got := acceptancetest.Exec(t, bin, t.TempDir(), nil, planCommand)
			expect.Equal(t, got.Status, cli.StatusOK, "plan exits 0")
			expect.HasPrefix(t, string(got.Stdout), planLine, "the output is the composition")
			expect.Empty(t, string(got.Stderr), "plan reports nothing")
		})

		t.Run("passes the environment of the test process with the variables of env", func(t *testing.T) {
			t.Parallel()

			self, err := os.Executable()
			assert.NoError(t, err, "the test binary has a path")
			added := []string{probeEnv + "=" + probeValue, replacedVar + "=" + replacedValue}
			got := acceptancetest.Exec(t, self, t.TempDir(), added)
			var env []string
			assert.NoError(t, json.Unmarshal(got.Stdout, &env), "the test binary writes its environment")
			inherited := slices.DeleteFunc(os.Environ(), func(kv string) bool {
				return strings.HasPrefix(kv, replacedVar+"=")
			})
			assert.Permutation(t, env, append(inherited, added...),
				"the process has the environment of the test with the variables of env in place of the inherited ones")
		})

		t.Run("stops the check for a binary that does not start", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			absent := filepath.Join(dir, absentName)
			records := assert.Rejects(t, "a binary that does not exist does not start", func(tb assert.TB) {
				acceptancetest.Exec(tb, absent, dir, nil)
			})
			assert.Length(t, records, 1, "the check fails once")
			assert.Equal(t, records[0].Contract, "the process of "+absent+" starts",
				"the record has the path of the binary")
		})
	})
}
