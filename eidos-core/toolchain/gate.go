// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"fmt"
	"os"
	"strconv"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
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
// cleanup the caller defers. A missing adapter, a fixture with no
// output and a layout the language refused each record a failure
// through tb, as the assertion of expect it states, and return an
// empty directory with a cleanup that does nothing.
func Prepare(tb assert.TB, a Adapter, g Generated) (string, func()) {
	tb.Helper()

	expect.NotNil(tb, a, "toolchain: an adapter lays the fixture out")
	if a == nil {
		return "", func() {}
	}
	lang := string(a.Lang())
	expect.NotEmpty(tb, g.Files, lang+": the fixture carries generated output, without which a toolchain "+
		"run proves nothing")
	if g.IsEmpty() {
		return "", func() {}
	}
	dir, err := a.Layout(g)
	expect.NoError(tb, err, lang+": the generated output lays out for the toolchain")
	if err != nil {
		return "", func() {}
	}
	return dir, func() { _ = os.RemoveAll(dir) }
}

// TB is the seat the gate reports through: the assert module's,
// widened with the skip a suite needs. The assertions take the assert
// module's seat alone, because they never skip. A failure is a record
// of the assertion that states it, which [assert.Rejects] and an
// [assert.Recorder] return.
type TB interface {
	assert.TB
	// Skip ends the test as skipped, with args as the reason.
	Skip(args ...any)
}

// Require reports whether the toolchain is there to run, and
// settles the absent case: locally it skips, naming the reason the
// adapter gave, and in CI it records a failure, so a regression cannot
// hide behind a missing compiler. A missing adapter records a failure
// and reports false.
//
// [RunToolchainSuite] calls it once for the kernel's two checks that
// run a toolchain. A satellite adding assertions of its own calls it
// too, because every caller that runs a toolchain passes the gate,
// whether or not the suite runs it.
func Require(tb TB, a Adapter) bool {
	tb.Helper()

	expect.NotNil(tb, a, "toolchain: an adapter answers whether its toolchain is present")
	if a == nil {
		return false
	}
	held, why := a.Available()
	if held {
		return true
	}
	if RequiredInCI() {
		expect.True(tb, held, string(a.Lang())+": the toolchain CI requires is present: "+why)
		return false
	}
	tb.Skip(fmt.Sprintf("%s: the toolchain is absent, so the assertion is skipped here and required in CI: %s",
		a.Lang(), why))
	return false
}
