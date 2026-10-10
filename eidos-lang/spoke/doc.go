// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spoke contains what the spokes of the cross-language hub
// share. A spoke spells a canonical type shape in its target language,
// and it refuses a shape without a spelling in the target. [Child] and
// [Children] spell the children of a shape through a spoke. The refusal
// of a child starts with the source spelling of the child. [Describe]
// returns the noun phrase that a refusal uses for the form of a shape.
//
// # Dependency position
//
// The package imports the sdk's emit model, rules and symbol vocabulary,
// and the Go stdlib.
package spoke
