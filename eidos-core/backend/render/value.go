// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"errors"
	"fmt"

	"go.dokimi.dev/eidos/core/diag"
)

// ValueError is the error a language's scaffold returns for a
// value it cannot spell, so the render reports it under
// [UnspeltValue] rather than as a template refusal.
//
// A caller reaches the classification through [errors.As], the way
// the store's refusals are read, which is what keeps the code
// load-bearing rather than a string a reader matches on.
type ValueError struct {
	// Lang names the target that refused, so the message reads the
	// way every other refusal in that language does.
	Lang string
	// Msg is one sentence naming the value and the refusal.
	Msg string
}

// Error renders the refusal.
func (e *ValueError) Error() string { return e.Lang + ": " + e.Msg }

// RefuseValue returns the error a scaffold refuses a value with.
func RefuseValue(lang, format string, args ...any) error {
	return &ValueError{Lang: lang, Msg: fmt.Sprintf(format, args...)}
}

// refusalCode classifies a render error: a value the language
// cannot spell reports under its own code, everything else as the
// template refusal it arrived as.
func refusalCode(err error) diag.Code {
	if _, refused := errors.AsType[*ValueError](err); refused {
		return UnspeltValue
	}
	return RefusedTemplate
}
