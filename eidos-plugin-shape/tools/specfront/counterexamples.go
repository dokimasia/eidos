// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

// Counterexamples are the inputs that a check of a classification
// covers. A spec states at least one.
type Counterexamples struct {
	Invalid string `yaml:"invalid" doc:"An input that the subject refuses."`
	Unsafe  string `yaml:"unsafe"  doc:"An input that does not pass through the subject unchanged."`
	Edge    string `yaml:"edge"    doc:"An input at a boundary of the claim."`
	Refused string `yaml:"refused" doc:"A callable or a use that the classification excludes."`
}
