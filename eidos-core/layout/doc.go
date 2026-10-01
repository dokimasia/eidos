// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package layout routes a settled plan's emitted declarations to the
// files they are written in.
//
// [Route] runs once per plan, after the settle and before the render.
// It reads four inputs in fixed precedence: the families each plugin
// declares, the plan's [Policy], the refinements of its [Config], and
// the overrides an author wrote on a declaration's origin. The target
// takes part through two backend surfaces: a [plugin.FileSpeller]
// splits units and spells filenames, and a [plugin.Packager] names the
// package a file at a routed path declares. The result is the plan's
// [plugin.File] list, which the render pass renders as it stands.
//
// [Residents] and [Modules] read the source tree a package rule
// consults, once per run from the frozen graph and the fact store, so
// every plan routes against one view of it.
//
// # Precedence
//
// Each decision takes the first input that states a value:
//
//   - family: tag= on the plugin's own directive, the kernel out
//     directive's tag, the family the handler addressed;
//   - directory: out= on the plugin's own directive, the kernel out
//     directive's path, the family's refinement, the generator's
//     refinement, the plan;
//   - filename: the same path overrides, the family's File, the
//     generator's File, the target's spelling;
//   - policy: the family's refinement, the generator's refinement, the
//     plan, [PolicyAlongside].
//
// A tag moves only a declaration of its plugin's primary family. A
// path override resolves against the directory of the origin's source
// file: a trailing slash, "." or ".." names a directory, and otherwise
// the last element is the filename.
//
// # Findings
//
// Route reports every routing problem as a positioned Error under
// [diag.PhaseLayout], and leaves the refused declarations out of the
// files it returns: [UndeclaredFamily], [UnknownTag],
// [AmbiguousOverride], [NoDestination], [EscapingPath],
// [PathCollision] and [UnderivedPackage]. A contradiction in the
// configuration is a fault [Config.Check] returns at Build, and never a
// finding.
//
// # Dependency position
//
// core/layout imports core/diag, core/directive, core/emit, core/meta,
// core/plugin, core/position, core/store, core/symbol,
// core/internal/pathset and the Go stdlib. The workspace imports it to
// route each plan, and nothing below the workspace does.
package layout
