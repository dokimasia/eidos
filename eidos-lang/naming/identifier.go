// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

// IsIdentifier reports whether s satisfies the ASCII identifier
// shape: letters, digits and underscores, not opening with a
// digit. A backend refusing to respell wire names tests with this
// before any convention runs, because a name outside the shape has
// no spelling a convention could honestly produce.
func IsIdentifier(s string) bool {
	return s != "" && isValidIdentifier(s)
}

// isValidIdentifier reports whether s already satisfies everything
// isValidIdentifier scans a non-empty s for the ASCII identifier
// shape. Deliberately ASCII-only: a rune outside the set fails,
// which for the wire-name rule is the honest answer — a spelling
// the scan cannot vouch for refuses rather than passing on a
// guess.
func isValidIdentifier(s string) bool {
	if s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		default:
			return false
		}
	}
	return true
}
