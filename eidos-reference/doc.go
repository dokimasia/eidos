// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package reference is the eidos module for the reference plugin
// ensemble: an API-typical consumer of the kernel's authoring
// surface, kept ordinary so that it exercises what real plugins use,
// the way real plugins use it.
//
// # Scope
//
// The ensemble is a consumer, not a facility. It consumes
// directives, slots, templates, exports and multi-plan composition,
// and it publishes nothing a plugin would import. The module
// contains this statement of scope and no code.
//
// # Dependency position
//
// The package imports nothing. Its test imports the assert module.
package reference
