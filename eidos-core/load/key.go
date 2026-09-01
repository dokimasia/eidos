// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"crypto/sha256"
	"encoding/binary"

	"go.dokimi.dev/eidos/core/node"
)

// unitKey folds one unit's key, by value, in the stated order: the
// unit's recorded reads, the partition's recorded reads, the
// depth, the frontend's declared version, the frontend's options
// in their canonical encoding, the composition's plugin-set
// fingerprint, and the node model's fingerprint. The order is part
// of the contract, because a key derived two ways diverges.
//
// Each part is length-prefixed, so two parts cannot trade bytes
// and collide.
func unitKey(u *unit, pluginSet []byte) []byte {
	h := sha256.New()
	part := func(b []byte) {
		var n [8]byte
		binary.LittleEndian.PutUint64(n[:], uint64(len(b)))
		h.Write(n[:])
		h.Write(b)
	}
	part(u.src.ReadSum())
	part(u.partition)
	part([]byte{byte(u.depth)})
	part([]byte(u.version))
	part(u.config)
	part(pluginSet)
	part([]byte(node.ModelFingerprint))
	return h.Sum(nil)
}
