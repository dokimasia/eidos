// Copyright Dokimasia B.V. 2026
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
// [ParsePolicy] parses the name of a [Policy] in a configuration, which is
// the name that [Policy.String] returns.
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
// # References
//
// Route sets the package of a reference that crosses into a file of
// another package, so the target imports the file. A bare reference has
// neither a target nor a package. It refers to a declaration of its
// unit's package by the declaration's settled name. A translated
// reference has a target of another language than the plan's
// [Input.Target]. It refers to the declaration of the plan whose origin
// is its target and whose settled name is its spelling.
//
// # Findings
//
// Route reports every routing problem as a positioned Error under
// [diag.PhaseLayout], and leaves the refused declarations out of the
// files it returns: [UndeclaredFamily], [UnknownTag],
// [AmbiguousOverride], [NoDestination], [EscapingPath],
// [PathCollision], [UnderivedPackage] and [UntranslatedReference]. A
// contradiction in the configuration is a fault [Config.Check] returns
// at Build, and never a finding.
//
// # Dependency position
//
// core/layout imports core/diag, core/directive, core/emit, core/meta,
// core/plugin, core/position, core/store, core/symbol,
// core/internal/pathset and the Go stdlib. The workspace imports it to
// route each plan, and nothing below the workspace does.
package layout
