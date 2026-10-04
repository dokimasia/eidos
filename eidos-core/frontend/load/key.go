// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"crypto/sha256"
	"encoding/binary"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
)

// unitKey returns the key of the unit whose fold [appendFold] returned:
// the SHA-256 digest of the fold. The key is its one allocation.
func unitKey(fold []byte) []byte {
	sum := sha256.Sum256(fold)
	return sum[:]
}

// appendFold appends one unit's fold to dst, by value, in the stated
// order, and returns the extended buffer: the number of members, then
// each member's path and digest, its number of shared inputs, and each
// shared input's path and digest, in roster order; then the fold of the
// door that returned the unit, the depth, the frontend's name, language
// and declared version, the frontend's options in their canonical
// encoding, the composition's brand, and the node model's fingerprint.
//
// The fold is computed from the gate's digests before the parse. The
// unit's one door refuses a path outside its members and their shared
// inputs, so the key covers each byte the parse can read. The name and
// language fold because both shape the identities the unit produces,
// and the brand because a parse reads a comment line as a carrier by
// the brand's marks. The version's bump-on-any-graph-change rule covers
// the comment syntax. Each part is length-prefixed and each list
// counted, so two rosters cannot trade bytes and collide.
//
// appendFold allocates only where dst lacks the room for the fold.
//
// Error modes: the error of a member or a shared input whose digest
// the gate cannot compute, because the file does not read.
func appendFold(dst []byte, u *unit, g *gate, brand output.Brand) ([]byte, error) {
	dst = appendCount(dst, len(u.files))
	for _, ref := range u.files {
		var err error
		if dst, err = appendFile(dst, g, ref.Path); err != nil {
			return dst, err
		}
		dst = appendCount(dst, len(ref.Shared))
		for _, shared := range ref.Shared {
			if dst, err = appendFile(dst, g, shared); err != nil {
				return dst, err
			}
		}
	}
	dst = appendPart(dst, u.door)
	dst = appendPart(dst, []byte{byte(u.depth)})
	dst = appendPart(dst, u.frontend.Name())
	dst = appendPart(dst, u.frontend.Lang())
	dst = appendPart(dst, u.version)
	dst = appendPart(dst, u.config)
	dst = appendPart(dst, brand)
	return appendPart(dst, node.ModelFingerprint), nil
}

// appendFile appends one file's path and the digest of its bytes. A path
// no file is at takes an empty part in place of the digest, so a shared
// input that appears later changes the keys of the units that declare
// it.
func appendFile(dst []byte, g *gate, path string) ([]byte, error) {
	digest, present, err := g.digest(path)
	if err != nil {
		return dst, err
	}
	dst = appendPart(dst, path)
	if !present {
		return appendPart(dst, []byte(nil)), nil
	}
	return appendPart(dst, digest[:]), nil
}

// appendCount appends a list's length as a part: its unsigned varint.
func appendCount(dst []byte, n int) []byte {
	var count [binary.MaxVarintLen64]byte
	return appendPart(dst, count[:binary.PutUvarint(count[:], uint64(n))])
}

// appendPart appends one field behind its length, eight bytes in little
// endian order.
func appendPart[B ~string | ~[]byte](dst []byte, b B) []byte {
	dst = binary.LittleEndian.AppendUint64(dst, uint64(len(b)))
	return append(dst, b...)
}
