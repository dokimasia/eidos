// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package render is the pass every backend runs: the procedure every
// language shares, owned once.
//
// [New] composes a [Language], the handful of things a target
// genuinely varies in, into a [Pass]. [Pass.Render] takes the files
// a plan's layout routed through the fixed steps: render declarations
// through the kind templates in the canonical order the flush fixed,
// finalise each file through the language formatter, and return files
// as values. The layout groups units into those files through the
// language's [Naming] and [Split], which the pass serves as
// [Pass.FileName] and [Pass.SplitUnit]. Nothing arrives on disk:
// staging, headers and trailers belong to the output contract that
// consumes the values.
//
// # Failure semantics
//
// A problem with one file or one declaration attaches to the sink
// as a positioned Error, at the routed file's path, and the pass
// continues: a kind the language cannot spell skips that declaration,
// and a file the formatter refuses is withheld while its siblings
// render whole. A finding outside every file, such as one from the
// vocabulary merge, is positioned at the pass's own name. Findings
// arrive in file order, whatever order the workers finish in. A
// returned error is a defect in the pass's own inputs. Nothing here
// panics.
//
// # Dependency position
//
// core/backend/render imports core/plugin, core/emit, core/diag,
// core/position, core/symbol and the Go stdlib, text/template
// included. It never imports the root authoring package: the
// backend kit there lowers to this pass, so the pass works
// directly on the SPI.
package render
