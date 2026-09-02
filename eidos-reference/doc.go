// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package reference is the module the reference plugin ensemble
// takes: an API-typical consumer of the kernel's authoring
// surface, kept deliberately ordinary so that what it exercises is
// what real plugins use, the way real plugins use them.
//
// # Scope
//
// The ensemble's purpose is to be a consumer rather than a
// facility: it consumes directives, slots, templates, exports and
// multi-plan composition, and it publishes nothing a plugin would
// import. The module holds this statement of scope and no code.
package reference
