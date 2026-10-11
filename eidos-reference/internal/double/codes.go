// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package double

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that the plugins of the ensemble
// share. The package internal/callee registers the first code under it.
const CodePrefix diag.Prefix = "REFERENCE"

var (
	// IncompleteMembers reports an interface whose members the rules of
	// its language cannot list in full, such as one that embeds an
	// interface that the load did not resolve. The finding is an Error at
	// the interface, and the plugin writes no double of it.
	IncompleteMembers = diag.MustRegister(CodePrefix, diag.CodeSpec{
		Number:  2,
		Meaning: "a double cannot implement an interface whose members are incomplete",
	})
	// Property reports an interface with a property. A double implements
	// the methods of its interface, so the finding is an Error at the
	// interface, and the plugin writes no double of it.
	Property = diag.MustRegister(CodePrefix, diag.CodeSpec{
		Number:  3,
		Meaning: "a double cannot implement a property of its interface",
	})
)
