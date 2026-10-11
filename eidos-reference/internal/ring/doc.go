// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command ring tests the published version of each module of the canary
// ring of eidos against the kernel and the SDK facade of a checkout.
//
// CI tests every module of the workspace at the commit of the checkout. A
// published module of the ring, such as a satellite or eidos-reference,
// also runs with a kernel that is newer than the kernel of its release.
// ring builds that combination:
//
//  1. It reads the go.work of the checkout, and lists each module that
//     requires the kernel, go.dokimi.dev/eidos/core. The kernel and the
//     SDK facade, go.dokimi.dev/eidos/sdk, are not in the ring.
//  2. It queries the module proxy for the latest version of each module of
//     the ring, as go list -m <module>@latest does.
//  3. For a release or a pre-release, it writes a scratch module that
//     requires that version, and replaces the kernel and the SDK facade
//     with their directories in the checkout. It then runs go test over
//     the packages of the module.
//
// A module is unpublished when the proxy returns a pseudo-version, as it
// does for a module without a release tag, or when the proxy answers 404
// Not Found or 410 Gone. ring does not test an unpublished module. Any
// other error of the query fails the module, so a proxy that does not
// answer fails the run.
//
// # Usage
//
//	go run ./eidos-reference/internal/ring [-root dir]
//
// -root is the directory of the go.work of the checkout, and the working
// directory by default. The go command reads its settings, such as
// GOPROXY, from the environment of the ring, and runs without the
// workspace of the checkout.
//
// # Output
//
// ring writes one line for each module of the ring to standard output, in
// the order of the module paths:
//
//	ok <module>@<version>
//	FAIL <module>@<version>: <error>
//	FAIL <module>: <error>
//	unpublished <module>: <reason>
//
// The output of go test follows the line of a module whose test fails. The
// second form of FAIL reports a query that failed.
//
// ring exits with status 0 when no module fails, and with status 1 when a
// module fails or the scratch directory of the run cannot be created. It
// exits with status 2 for a command line or a checkout that it cannot
// read.
//
// # Dependency position
//
// The package imports golang.org/x/mod/modfile, golang.org/x/mod/module
// and the Go stdlib. It does not import a package of eidos.
package main
