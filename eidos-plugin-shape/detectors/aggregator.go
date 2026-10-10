// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors

import "go.dokimi.dev/eidos/sdk/rules"

// Aggregator reports a callable that takes no input and returns one value,
// with or without a context and an error model.
func Aggregator(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.inputs == 0 && s.values == 1
}

// MultiAggregator reports a callable that takes no input and returns two
// or more values beside its error.
func MultiAggregator(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.fails && s.inputs == 0 && s.values >= 2
}

// StreamConsumer reports a callable that takes a context and one stream,
// which is an input of an interface type, and returns one value beside
// its error.
func StreamConsumer(c rules.Callable, b rules.Bound) bool {
	s := signatureOf(c)
	return s.context && s.fails && s.inputs == 1 && s.values == 1 && isInterface(b, s.input.Ref)
}

// Computation reports a callable that returns one value and has no
// context and no error model, whatever it takes.
func Computation(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return !s.context && !s.fails && len(c.Returns) == 1
}
