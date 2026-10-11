// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package callee contains the rules of the plugins of the ensemble that
// call a function of another package. The options of each plugin set the
// function.
//
// Each rule appends one statement into a body that another plugin wrote:
//
//   - [Chain] wraps the handler of a middleware function in one middleware,
//     in the named slot of the chain.
//   - [Epilogue] calls a function of the request at the end of the method
//     ServeHTTP of a handler.
//   - [Weave] logs the entry of each method with a receiver in its
//     prologue.
//
// The call of each statement refers to the function by its identity, and
// the target qualifies the call and imports the package.
//
// # Diagnostics
//
// The options contract of a plugin has no required key. So [Check] reports
// [Missing] at the subject where the options leave the package or the
// function empty, and a rule then appends nothing. A plugin that calls the
// function through a template checks its options with Check too.
//
// # Dependency position
//
// The package imports the packages handlergen and middlewaregen of the
// module, the root of the Go satellite, and the root, diag, emit and symbol
// packages of the SDK facade.
package callee
