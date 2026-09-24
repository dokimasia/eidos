// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"fmt"
	"os"
	"strconv"
)

// ciEnv is the variable every runner in use sets, and the one thing
// that decides whether a missing toolchain is a skip or a failure.
const ciEnv = "CI"

// RequiredInCI reports whether a missing toolchain must fail rather
// than skip.
//
// It reads the CI variable, which every runner in use sets, so a
// contributor without a language's compiler still runs the rest of
// the suite while the same suite in CI covers what they skipped.
// An empty value and a false boolean, such as CI=false, run as
// local. Any other value runs as CI.
func RequiredInCI() bool {
	v := os.Getenv(ciEnv)
	if v == "" {
		return false
	}
	set, err := strconv.ParseBool(v)
	return err != nil || set
}

// Prepare lays a fixture out and returns its directory and the
// cleanup the caller defers. A fixture carrying no output and a
// layout the language refused are both failures, reported through
// tb, and the returned directory is empty.
func Prepare(tb TB, a Adapter, g Generated) (string, func()) {
	tb.Helper()

	if a == nil {
		tb.Errorf("toolchain: no adapter to lay the fixture out with")
		return "", func() {}
	}
	if g.IsEmpty() {
		tb.Errorf("%s: the fixture carries no generated output, so a toolchain run proves nothing", a.Lang())
		return "", func() {}
	}
	dir, err := a.Layout(g)
	if err != nil {
		tb.Errorf("%s: laying the generated output out for the toolchain: %v", a.Lang(), err)
		return "", func() {}
	}
	return dir, func() { _ = os.RemoveAll(dir) }
}

// TB is the test handle the assertions report through: the assert
// module's, widened with the skip a suite needs, so one interface
// serves an assertion and the suite that gates it.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Skip(args ...any)
}

// Require reports whether the toolchain is there to run, and
// settles the absent case: locally it skips, naming the reason the
// adapter gave, and in CI it fails, so a regression cannot hide
// behind a missing compiler.
//
// [RunToolchainSuite] calls it once for the kernel's three checks.
// A satellite adding assertions of its own calls it too, because
// the gate belongs to whoever runs a toolchain rather than to the
// suite.
func Require(tb TB, a Adapter) bool {
	tb.Helper()

	if a == nil {
		tb.Errorf("toolchain: no adapter to ask about a toolchain")
		return false
	}
	held, why := a.Available()
	if held {
		return true
	}
	if RequiredInCI() {
		tb.Errorf("%s: the toolchain is required in CI and is absent: %s", a.Lang(), why)
		return false
	}
	tb.Skip(fmt.Sprintf("%s: the toolchain is absent, so the assertion is skipped here and required in CI: %s",
		a.Lang(), why))
	return false
}
