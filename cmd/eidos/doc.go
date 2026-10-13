// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command eidos generates the code of a workspace with the declarative
// plugins that its config file adds to the plans of the binary.
//
// The binary composes the frontends of Go, TypeScript, Java, Rust and
// protobuf, the rules of Go, TypeScript and protobuf, and one extensible
// plan for each of the targets golang, typescript, java and rust. It has
// no generator and no annotator of its own. A plan without a plugin writes
// no file.
//
// # The config file
//
// The config file is .eidos.yaml at the workspace root, and the state
// directory is .eidos/. A source file sets the directive of a plugin as
// +eidos:<directive>. The key plugins of a plan lists the directories of
// its declarative plugins, and the key sources replaces the packages that
// the plan reads:
//
//	version: 1
//	plans:
//	    golang:
//	        plugins: [plugins/audit]
//	    typescript:
//	        sources: {lang: golang, packages: [./ops]}
//	        plugins: [plugins/audit]
//
// Each plan reads the packages of the language of its target below the
// workspace root unless the config file replaces its sources.
//
// # Stores
//
// A run locates the stores of a frontend only when the tree has an input
// file of the frontend. A tree with a Go file needs GOROOT or a go binary
// on PATH, and a tree with a Java file needs JAVA_HOME.
//
// # Dependency position
//
// The command imports the command kernels of eidos-cli, core/output,
// core/workspace, and the frontend, backend and rules packages of the Go,
// TypeScript, Java, Rust and protobuf satellites.
package main
