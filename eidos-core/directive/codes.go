// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package directive

import "go.dokimi.dev/eidos/core/diag"

// The codes validation refuses under, one per failure class so a
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
		Number: 6, Meaning: "a directive carries a key its schema does not accept",
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
		Number: 10, Meaning: "a directive carries more positional arguments than its schema declares",
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
	// directive the subject does not carry.
	RequirementUnmet = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 15, Meaning: "a directive requires another the subject does not carry",
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
	// DanglingSubject refuses a directive attached to a subject
	// the graph never got. The check lives with whoever holds the
	// graph; the code lives with its class.
	DanglingSubject = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 18, Meaning: "a directive is attached to a subject the graph does not hold",
	})
	// UnsealedRegistry refuses validation against a registry still
	// registering: a defect in the composition, not in a carrier.
	UnsealedRegistry = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number: 19, Meaning: "directive validation ran before the registry sealed",
	})
)
