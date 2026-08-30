// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag

import "strconv"

// Severity says what a finding means for the run.
//
// The zero value is [SeverityError]. A finding that answered no
// severity is a defect in whatever reported it, and a defect must
// not read as one of the severities a run tolerates.
type Severity uint8

const (
	// SeverityError means the run is wrong: a refusal, a violated
	// contract, a collision. Any Error fails the run.
	SeverityError Severity = iota
	// SeverityWarning means the output stands but a human should
	// look. A Warning never fails a run.
	SeverityWarning
	// SeverityInfo carries provenance and progress.
	SeverityInfo
)

// severityNames spells each severity, indexed by the severity
// itself.
var severityNames = [...]string{
	SeverityError:   "error",
	SeverityWarning: "warning",
	SeverityInfo:    "info",
}

// String answers the severity's spelling.
//
// The spellings are what machine output carries, so a consumer
// matching on them matches on API. A severity nothing declares
// answers its number rather than a name.
func (s Severity) String() string {
	if int(s) >= len(severityNames) {
		return "Severity(" + strconv.Itoa(int(s)) + ")"
	}
	return severityNames[s]
}
