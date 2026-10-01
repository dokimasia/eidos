// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import "strconv"

// Policy is where a plan places a generated file by default.
type Policy uint8

const (
	// PolicyInherit takes the policy of the enclosing scope: a
	// refinement takes its plan's, and a plan takes PolicyAlongside.
	PolicyInherit Policy = 0
	// PolicyAlongside places a file in the directory of the source it
	// derives from.
	PolicyAlongside Policy = 1
	// PolicyCentralised places a file under the configured output
	// directory, at its source package's workspace-relative directory.
	PolicyCentralised Policy = 2
)

// Valid reports whether p is one of the three policies.
func (p Policy) Valid() bool { return p <= PolicyCentralised }

// String returns the policy's spelling in a configuration. A policy
// outside the three returns its number.
func (p Policy) String() string {
	switch p {
	case PolicyInherit:
		return "inherit"
	case PolicyAlongside:
		return "alongside-source"
	case PolicyCentralised:
		return "centralised"
	default:
		return "Policy(" + strconv.Itoa(int(p)) + ")"
	}
}

// or returns p, and fallback where p inherits.
func (p Policy) or(fallback Policy) Policy {
	if p == PolicyInherit {
		return fallback
	}
	return p
}
