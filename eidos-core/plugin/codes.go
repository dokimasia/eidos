// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import "go.dokimi.dev/eidos/core/diag"

// ContinuedCarrier refuses a carrier that continues onto the next
// line while its own line has the tool-directive shape. A formatter
// may move such a line to the end of its doc comment, as gofmt does,
// which separates the carrier from its continuation and drops the
// continued arguments without a finding. The set mark never has the
// shape, so a continued carrier written with it keeps its place.
var ContinuedCarrier = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 44, Meaning: "a carrier in the tool-directive shape continues onto the next line",
})
