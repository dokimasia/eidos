// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package acceptancetest_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/eidos/cli/acceptancetest"
	"go.dokimi.dev/eidos/core/output"
)

// The main packages of the hosts that the tests build. host meets the
// contract, and each other host breaks one rule of it.
const (
	hostMain       = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/host"
	remappedMain   = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/remapped"
	renamedMain    = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/renamed"
	recoveringMain = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/recovering"
	pinnedMain     = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/pinned"
	bannerMain     = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/banner"
	clockMain      = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/clock"
	mutedMain      = "go.dokimi.dev/eidos/cli/acceptancetest/testdata/muted"
)

// brand is the brand of the hosts.
const brand output.Brand = "acme"

// The command of the hosts that panics, and the status of a panic.
const (
	crashName   = "crash"
	panicStatus = 2
)

// The files of the tree of the fixture: the config file of the brand, the
// directory and the scripted source of the store, the mirror that a run
// writes beside it, and the declaration that the mirror has.
const (
	configName   = ".acme.yaml"
	storeDir     = "svc"
	storePath    = storeDir + "/store.zz"
	mirrorName   = "mirror.txt"
	mirroredDecl = "type ForStore struct{}"
)

// The file that the Fail of the fixture writes: a scripted file without a
// package line, which the scripted frontend reports as an Error.
const (
	badName   = "bad.zz"
	badSource = "type Bad string\n"
	fileMode  = fs.FileMode(0o644)
)

// errUncompiled is the error of the Compile of the fixture for a mirror
// without the declaration of the store.
var errUncompiled = errors.New("acceptancetest_test: the mirror has no declaration of the store")

// tree is the tree of the fixture: the config file of the brand and the
// source of the store.
var tree = fstest.MapFS{
	configName: {Data: []byte("version: 1\n")},
	storePath:  {Data: []byte("package svc\ntype Store string\n")},
}

// The suite checks the binary of a host that meets the contract.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunAcceptanceSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the binary of a host that meets the contract", func(t *testing.T) {
			t.Parallel()

			acceptancetest.RunAcceptanceSuite(t, fixture())
		})
	})
}

// fixture returns the fixture of the host that meets the contract. Its Fail
// writes a scripted file without a package line, and its Compile reads the
// mirror of the store.
func fixture() acceptancetest.Fixture {
	return acceptancetest.Fixture{
		Main:  hostMain,
		Brand: brand,
		Tree:  tree,
		Panic: []string{crashName},
		Fail: func(root string) error {
			return os.WriteFile(filepath.Join(root, storeDir, badName), []byte(badSource), fileMode)
		},
		Compile: compile,
	}
}

// compile is the Compile of the fixture. It reads the mirror of the store,
// and returns errUncompiled for a mirror without the declaration of the
// store.
//
// Error modes: the error of a mirror that does not read, and errUncompiled.
func compile(_ context.Context, root string) error {
	b, err := os.ReadFile(filepath.Join(root, storeDir, mirrorName))
	if err != nil {
		return err
	}
	if !bytes.Contains(b, []byte(mirroredDecl)) {
		return errUncompiled
	}
	return nil
}
