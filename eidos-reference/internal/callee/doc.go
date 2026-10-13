// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package callee contains the rules of the plugins of the ensemble that
// call a function of another package. The options of each plugin set the
// function.
//
// Each rule appends statements into bodies that another plugin wrote, and
// its trigger and its value gate select those bodies:
//
//   - [Chain] wraps the handler of a middleware function in one middleware,
//     in the named slot of the chain.
//   - [Epilogue] calls a function of the request at the end of the method
//     ServeHTTP of a handler.
//   - [Weave] logs the entry of each method of an emitted struct in its
//     prologue.
//
// A plugin wraps the rule in its own gate on the origin. The call of each
// statement refers to the function by its identity, and the target
// qualifies the call and imports the package.
//
// # Diagnostics
//
// The package and the function of [Options] are required options, so a
// composition refuses an empty one at Build. The weavers set a default for
// each option of [Log]. [Check] reports [Missing] at the subject for an
// empty option of a plugin that runs outside a composition, such as in a
// fixture, and a rule then appends nothing. Check also resolves the options
// against the load. It reports [UnknownFunction] where the package does not
// declare a function of the name, and [UnknownPackage] where the module of
// the subject does not contain a package at the path. Each rule checks its
// options once for each invocation. A plugin that calls the function
// through a template checks its options with Check too.
//
// # Dependency position
//
// The package imports the packages handlergen and middlewaregen of the
// module, the root of the Go satellite, the root, diag, emit, meta, node,
// rules and symbol packages of the SDK facade, and the Go stdlib.
package callee
