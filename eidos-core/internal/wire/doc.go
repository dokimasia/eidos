// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package wire reads and writes the kernel's binary records: a
// sequence of values with no offset and no length of a whole record.
//
// An unsigned integer is a uvarint and a signed one a varint, as
// [encoding/binary] appends them. A bool is one byte, zero or one. A
// byte string and a text are a uvarint length and then the bytes, as
// [AppendBytes] and [AppendText] append them. A list is a uvarint count
// and then its elements, each at least one byte long. A SHA-256 digest
// is its 32 bytes.
//
// [Decoder] reads the same values in sequence and refuses a malformed
// encoding: the bytes end inside a value, an integer overflows, a bool
// is neither zero nor one, or a length exceeds the bytes left. Its
// first failure ends the decode and wraps [ErrMalformed], so a caller
// reads a whole record and checks one error.
//
// # Dependency position
//
// core/internal/wire imports the Go stdlib alone.
package wire
