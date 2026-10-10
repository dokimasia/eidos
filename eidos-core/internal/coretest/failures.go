// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import "go.dokimi.dev/assert"

// Contracts returns the contract of each failure record, in the order
// of the records: what a case that runs a kit's check through
// [assert.Rejects] compares, so it pins which contracts the check broke
// and how often, and not the records' call sites.
//
// # Allocation contract
//
// Contracts allocates the list it returns, one allocation, and nothing
// for no record.
func Contracts(records []assert.Failure) []string {
	if len(records) == 0 {
		return nil
	}
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Contract
	}
	return out
}
