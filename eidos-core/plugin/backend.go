// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

// Target names a rendering target. It is a registered name: the
// composition declares the targets it recognises, and a plan whose
// backend names anything else is a composition fault. The zero
// Target names nothing.
type Target string

// Backend renders one plan's emit. A plan holds exactly one, and
// its target must be a registered name; a composition that runs no
// renderer still validates both, so a plan is whole before
// anything arrives on disk.
type Backend interface {
	Plugin
	Target() Target
}
