// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command eidos generates the code of a workspace with the declarative
// plugins that its config file adds to the plans of the binary.
//
// The binary composes the frontends of Go, TypeScript, Java, Rust, protobuf,
// GraphQL and Python, the rules of Go, TypeScript, Rust, protobuf, GraphQL
// and Python, and one extensible plan for each of the targets golang,
// typescript, java, rust, protobuf, graphql and python. It has no generator
// and no annotator of its own. A plan without a plugin writes no file.
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
// on PATH, and a tree with a Java file needs JAVA_HOME. A tree with a Python
// file reads the standard library from the stubs of typeshed that the
// binary embeds, and the installed distributions from the site-packages of
// the virtual environment that VIRTUAL_ENV names.
//
// # Dependency position
//
// The command imports the command kernels of eidos-cli, core/output,
// core/workspace, the root, frontend and backend packages of the Go,
// TypeScript, Java, Rust, protobuf, GraphQL and Python satellites, the rules
// packages of the Go, TypeScript, Rust, protobuf, GraphQL and Python
// satellites, and the Go stdlib.
package main
