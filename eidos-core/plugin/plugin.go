// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import (
	"strconv"

	"go.dokimi.dev/eidos/core/diag"
)

// ID is a plugin's declared name: the one identity everywhere it
// appears. The diagnostic origin, the emit attribution and the
// arbitration rank's plugin field all carry this same type, so no
// boundary converts. It aliases [diag.Origin] rather than
// defining its own type because those envelopes live beneath the
// service provider interface, where this package cannot be
// imported; the definition sits at the bottom and this is its
// spelling wherever plugins are the subject.
type ID = diag.Origin

// Plugin is the base contract: a stable name.
//
// The name never changes between versions, because everything
// durable keys on it: findings, units, claims, config sections and
// the composition's roster.
type Plugin interface {
	Name() ID
}

// Role names one phase role a plugin can hold.
//
// Priorities are per role: one shared number cannot place a
// dual-role plugin's annotator half and generator half
// independently, and the two phases' orderings have no reason to
// share integers. The zero Role names no role.
type Role uint8

const (
	// RoleAnnotator is the role that stamps facts over the frozen
	// graph.
	RoleAnnotator Role = iota + 1
	// RoleGenerator is the role that produces emit values into one
	// plan.
	RoleGenerator
)

// String returns the role's spelling. Faults and stats name roles,
// so a consumer matching on the spelling matches on API. A role
// nothing declares returns its number rather than a name.
func (r Role) String() string {
	switch r {
	case RoleAnnotator:
		return "annotator"
	case RoleGenerator:
		return "generator"
	default:
		return "Role(" + strconv.Itoa(int(r)) + ")"
	}
}

// Capability is a label one plugin provides and another requires.
//
// The provider exports it as a constant and requirers reference
// that constant, so a misspelled requirement is a compile error in
// a typed plugin and a collected fault only for what a manifest
// spelled. Capability topology orders execution inside one
// priority bucket.
type Capability string
