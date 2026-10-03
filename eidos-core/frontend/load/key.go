// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
)

// unitKey folds one unit's key, by value, in the stated order: the
// number of members, then each member's path and digest, its number of
// shared inputs, and each shared input's path and digest, in roster
// order; then the fold of the door that returned the unit, the depth,
// the frontend's name, language and declared version, the frontend's
// options in their canonical encoding, the composition's brand, and the
// node model's fingerprint.
//
// The key is computed from the gate's digests before the parse, and the
// unit's one door refuses every path outside its members and their
// shared inputs, so the key covers every byte the parse can read. The
// name and language fold because both shape every identity the unit
// produces, and the brand because its marks decide which comment lines
// are carriers. The version's bump-on-any-graph-change rule covers the
// comment syntax. Each part is length-prefixed and every list counted,
// so two rosters cannot trade bytes and collide.
//
// Error modes: the error of a member or a shared input whose digest
// the gate cannot compute, because the file does not read.
func unitKey(u *unit, g *gate, brand output.Brand) ([]byte, error) {
	h := sha256.New()
	part(h, binary.AppendUvarint(nil, uint64(len(u.files))))
	for _, ref := range u.files {
		if err := partFile(h, g, ref.Path); err != nil {
			return nil, err
		}
		part(h, binary.AppendUvarint(nil, uint64(len(ref.Shared))))
		for _, shared := range ref.Shared {
			if err := partFile(h, g, shared); err != nil {
				return nil, err
			}
		}
	}
	part(h, u.door)
	part(h, []byte{byte(u.depth)})
	part(h, []byte(u.frontend.Name()))
	part(h, []byte(u.frontend.Lang()))
	part(h, []byte(u.version))
	part(h, u.config)
	part(h, []byte(brand))
	part(h, []byte(node.ModelFingerprint))
	return h.Sum(nil), nil
}

// partFile folds one file's path and the digest of its bytes, and an
// empty part in place of the digest for a path no file is at, so a
// shared input that appears re-keys every unit that declares it.
func partFile(h hash.Hash, g *gate, path string) error {
	digest, present, err := g.digest(path)
	if err != nil {
		return err
	}
	part(h, []byte(path))
	if !present {
		part(h, nil)
		return nil
	}
	part(h, digest[:])
	return nil
}

// part writes one field into a key behind its length.
func part(h hash.Hash, b []byte) {
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(len(b)))
	h.Write(n[:])
	h.Write(b)
}
