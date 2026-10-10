// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

// Precedence lists the detected shapes ranked before a detected shape, for
// a callable that both detectors report.
type Precedence struct {
	YieldsTo []Name `yaml:"yields_to" schema:"required" doc:"The detected shapes that rank before this one."`
}
