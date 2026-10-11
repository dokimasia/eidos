// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package tree returns the template tree of one target from the templates
// that a plugin of the ensemble embeds.
//
// A plugin keeps the templates of each target that it serves in the
// directory templates/<target> of its package, and embeds the directory
// templates. [Of] returns the directory of one target as the tree that the
// plugin declares for the target, so a template of each target has the
// same name in its tree.
//
// # Dependency position
//
// The package imports the plugin package of the SDK facade and the Go
// stdlib.
package tree
