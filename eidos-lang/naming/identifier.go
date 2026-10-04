// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

// IsIdentifier reports whether s satisfies the ASCII identifier
// shape: letters, digits and underscores, not opening with a
// digit. A backend refusing to respell wire names tests with this
// before any convention runs, because a name outside the shape has
// no spelling a convention could honestly produce. It allocates
// nothing.
func IsIdentifier(s string) bool {
	return s != "" && isValidIdentifier(s)
}

// isValidIdentifier reports whether a non-empty s has the ASCII
// identifier shape. A rune outside ASCII fails the scan, because the
// wire-name rule refuses a spelling the scan cannot check.
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
