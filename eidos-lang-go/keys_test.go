// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/meta"
)

// One namespace claim serves both stamping roles, so the
// registration round is pinned: every key resolves, the handles
// come back typed, and a second claim refuses.
func TestKeys(t *testing.T) {
	t.Parallel()

	r := meta.NewRegistry()
	handles, err := golang.Register(r)
	assert.NoError(t, err, "the whole vocabulary registers")

	for _, key := range []meta.KeyName{
		golang.TestFileKey, golang.ConstraintKey, golang.GeneratedKey,
		golang.CgoKey, golang.TypeSetKey, golang.ConstraintInterfaceKey,
		golang.EmptyInterfaceKey, golang.ReceiverPointerKey,
		golang.UnderlyingKey, golang.IterSeqKey, golang.IterSeq2Key,
		golang.ConstValueKey, golang.SatisfiesErrorKey,
		golang.SatisfiesStringerKey, golang.EmbedsInterfaceKey,
		golang.ComparableKey,
	} {
		_, held := r.Resolve(key)
		assert.True(t, held, string(key)+" resolves")
	}
	assert.False(t, handles.SatisfiesError.IsZero(), "the annotator's handles come back")
	assert.False(t, handles.Comparable.IsZero(), "all of them")

	assert.HasError(t, golang.Keys(r), "a second claim on the namespace refuses")
}
