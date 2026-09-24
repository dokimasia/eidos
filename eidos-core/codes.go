// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import "go.dokimi.dev/eidos/core/meta"

// RefusedStamp is [meta.RefusedStamp], re-exported where annotator
// authors read. It reports a refused stamp at the subject's
// position under the stamping plugin's identity, and the phase
// continues. The code is registered in core/meta beside the fact
// store, because the classification path reports it too.
var RefusedStamp = meta.RefusedStamp
