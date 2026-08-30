// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import "strconv"

// Plugin is the base contract: a stable name.
//
// The name is the plugin's one identity everywhere it appears: the
// diagnostic origin, the emit attribution and the arbitration
// rank's plugin field. It never changes between versions, because
// everything durable keys on it.
type Plugin interface {
	Name() string
}

// Role names one phase seat a plugin can hold.
//
// Priorities are per role: one shared number cannot place a
// dual-role plugin's annotator half and generator half
// independently, and the two phases' orderings have no reason to
// share integers. The zero Role names no seat.
type Role uint8

const (
	// RoleAnnotator is the seat that stamps facts over the frozen
	// graph.
	RoleAnnotator Role = iota + 1
	// RoleGenerator is the seat that produces emit values into one
	// plan.
	RoleGenerator
)

// String answers the role's spelling. Faults and stats name roles,
// so a consumer matching on the spelling matches on API. A role
// nothing declares answers its number rather than a name.
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
