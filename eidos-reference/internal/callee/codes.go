// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package callee

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that the plugins of the ensemble
// share.
const CodePrefix diag.Prefix = "REFERENCE"

var (
	// Missing reports a plugin whose options leave the package or the
	// function of its call empty. The finding is an Error at the subject
	// of the rule, and the plugin appends nothing for it.
	Missing = diag.MustRegister(CodePrefix, diag.CodeSpec{
		Number:  1,
		Meaning: "the options of a plugin set no function for the plugin to call",
	})
	// UnknownPackage reports options that name a package of the module of
	// the subject that the load does not contain, such as a package that
	// was deleted. The finding is an Error at the subject of the rule, and
	// the plugin appends nothing for it.
	UnknownPackage = diag.MustRegister(CodePrefix, diag.CodeSpec{
		Number:  5,
		Meaning: "the options of a plugin name a package that the module of the subject does not contain",
	})
	// UnknownFunction reports options that name a function that their
	// package does not declare. The finding is an Error at the subject of
	// the rule, and the plugin appends nothing for it.
	UnknownFunction = diag.MustRegister(CodePrefix, diag.CodeSpec{
		Number:  6,
		Meaning: "the options of a plugin name a function that their package does not declare",
	})
)
