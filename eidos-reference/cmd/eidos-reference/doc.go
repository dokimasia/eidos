// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command eidos-reference runs the reference plugins of eidos over a
// workspace.
//
// The binary mounts the kernel commands of eidos, such as run, plan and
// explain, over [go.dokimi.dev/eidos/reference.Compose]. A workspace sets
// the directives of the ensemble under the brand reference, such as
// //+reference:handler, and the options of the plugins in its config file
// .reference.yaml:
//
//	version: 1
//	options:
//	    authgen: {package: example.com/shop/httpmw/auth, func: RequireAuth}
//
// The command line has the form eidos-reference [flags] <command> [flags]
// [arguments], as every binary of eidos has. eidos-reference help lists the
// commands.
//
// # Dependency position
//
// The package imports go.dokimi.dev/eidos/cli and the package reference of
// the module.
package main
