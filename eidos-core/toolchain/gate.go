// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"fmt"
	"os"
	"strconv"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
)

// ciEnv is the variable that every runner in use sets. Its value decides
// whether a missing toolchain is a skip or a failure.
const ciEnv = "CI"

// RequiredInCI reports whether a missing toolchain must fail rather
// than skip.
//
// It reads the CI variable, which every runner in use sets, so a
// contributor without the compiler of a language still runs the rest of
// the suite, and the same suite in CI runs what the contributor skipped.
// An empty value and a false boolean, such as CI=false, run as local.
// Any other value runs as CI.
func RequiredInCI() bool {
	v := os.Getenv(ciEnv)
	if v == "" {
		return false
	}
	set, err := strconv.ParseBool(v)
	return err != nil || set
}

// Prepare lays a fixture out and returns its directory and the
// cleanup the caller defers. It reads every file of the fixture's
// sources into the files that the adapter lays out, at the file's path
// in the sources. A generated file replaces a source file at the same
// path, and the adapter does not receive the sources. A missing
// adapter, a fixture with no output, sources that do not read and a
// layout the language refused each record a failure through tb, as the
// assertion of expect it states, and return an empty directory with a
// cleanup that does nothing.
func Prepare(tb assert.TB, a Adapter, g Generated) (string, func()) {
	tb.Helper()

	expect.NotNil(tb, a, "toolchain: an adapter lays the fixture out")
	if a == nil {
		return "", func() {}
	}
	lang := string(a.Lang())
	expect.NotEmpty(tb, g.Files, lang+": the fixture has generated output, without which a toolchain "+
		"run proves nothing")
	if g.IsEmpty() {
		return "", func() {}
	}
	files, err := g.laidOut()
	expect.NoError(tb, err, lang+": the sources of the fixture read")
	if err != nil {
		return "", func() {}
	}
	dir, err := a.Layout(Generated{Files: files, Module: g.Module})
	expect.NoError(tb, err, lang+": the generated output lays out for the toolchain")
	if err != nil {
		return "", func() {}
	}
	return dir, func() { _ = os.RemoveAll(dir) }
}

// TB is the test role that the gate reports through. It is the TB of the
// assert module with the Skip method that a suite needs. The assertions
// take the TB of the assert module, because they never skip. A failure
// is a record of the assertion that states it, which [assert.Rejects]
// and an [assert.Recorder] return.
type TB interface {
	assert.TB
	// Skip ends the test as skipped, with args as the reason.
	Skip(args ...any)
}

// Require reports whether the toolchain is present. Where it is absent,
// Require skips the test locally with the reason that the adapter gave,
// and records a failure in CI, so a regression cannot hide behind a
// missing compiler. A missing adapter records a failure and reports
// false.
//
// [RunToolchainSuite] calls it once, before the three checks of the kit.
// A satellite that adds assertions of its own calls it too, because every
// caller that runs a toolchain passes the gate, whether or not the suite
// runs it.
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
