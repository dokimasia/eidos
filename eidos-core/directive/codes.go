// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import "go.dokimi.dev/eidos/core/diag"

// The codes validation reports under, one per class of finding so a
// consumer scripts against the class it cares about. Each is
// declared where it is registered, so nothing drifts.
var (
	// UnclaimedName refuses a directive nothing registered.
	UnclaimedName = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 4, Meaning: "a directive names no registered schema",
	})
	// AmbiguousName refuses a bare spelling two plugins claim.
	AmbiguousName = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 5, Meaning: "a bare directive name has two claimants",
	})
	// UnknownKey refuses a key outside the schema's closure.
	UnknownKey = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 6, Meaning: "a directive writes a key its schema does not accept",
	})
	// DuplicateKey refuses a key written twice in one instance.
	DuplicateKey = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 7, Meaning: "a directive writes one key twice",
	})
	// TypeMismatch refuses a value of the wrong shape.
	TypeMismatch = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 8, Meaning: "a directive value has the wrong shape for its param",
	})
	// BadSpelling refuses a value outside its type's spelling.
	BadSpelling = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 9, Meaning: "a directive value is outside its type's spelling",
	})
	// ExtraPositional refuses an argument past the declared
	// positionals.
	ExtraPositional = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 10, Meaning: "a directive writes more positional arguments than its schema declares",
	})
	// MissingParam refuses an omitted required param.
	MissingParam = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 11, Meaning: "a directive omits a param its schema requires",
	})
	// UnknownRole refuses a role outside the declared set.
	UnknownRole = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 12, Meaning: "a directive's role is outside its schema's declared set",
	})
	// MissingRole refuses a bare instance of a schema that demands
	// a role.
	MissingRole = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 13, Meaning: "a directive omits the role its schema demands",
	})
	// DuplicateInstance refuses a second instance of a
	// single-instance schema.
	DuplicateInstance = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 14, Meaning: "a single-instance directive appears twice on one subject",
	})
	// RequirementUnmet refuses an instance whose schema requires a
	// directive the subject does not have.
	RequirementUnmet = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 15, Meaning: "a directive requires another directive the subject does not have",
	})
	// Conflict refuses a pair of directives a schema declares
	// incompatible.
	Conflict = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 16, Meaning: "two directives on one subject conflict",
	})
	// UnknownMetadataKey refuses a metadata reference no key or
	// group returns.
	UnknownMetadataKey = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 17, Meaning: "a directive names a metadata key or group nothing registered",
	})
	// DanglingSubject refuses a directive or a classification
	// stamp attached to a subject the graph never got. The code that
	// has the graph runs the check, and the code is declared here
	// with its class.
	DanglingSubject = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 18, Meaning: "a directive or stamp is attached to a subject the graph does not contain",
	})
	// UnsealedRegistry refuses validation against a registry still
	// registering: a defect in the composition, not in a carrier.
	UnsealedRegistry = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 19, Meaning: "directive validation ran before the registry sealed",
	})
	// UnresolvedReference refuses a source reference the resolver
	// bound to nothing, naming the spelling and the kind it was
	// read at.
	UnresolvedReference = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 40, Meaning: "a directive's reference param resolves to nothing",
	})
	// NegationRefused refuses a negated instance of a schema that
	// does not declare itself negatable.
	NegationRefused = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 43, Meaning: "a directive is negated, and its schema does not accept the negated form",
	})
	// MixedCarriers warns where one subject's instances of a
	// repeatable directive mix carriers in the tool-directive shape
	// with other carriers, which a formatter may reorder.
	MixedCarriers = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 45, Meaning: "a repeatable directive mixes carriers a formatter may move with carriers it keeps",
	})
)
