// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package catalog contains the plugins of the shape catalog. [Annotators]
// returns them for a composition.
//
// The plugin shape declares the directives shape, mixin and contract, each
// with one variant for each spec of its form. It stamps the facts of every
// directive instance. It runs the detectors of [Detections] over every
// function and method without a shape directive, and stamps the first
// detected shape in the order of precedence. The key
// [go.dokimi.dev/eidos/plugin/shape.KeyDetected] lists every detected
// shape of the callable.
//
// The plugin shapecheck validates every contract instance. Each role of an
// instance has a number of callables that the arity of the role admits.
//
// # Diagnostics
//
// Each finding of the catalog is an Error:
//
//   - [RoleArity] at a callable of a contract instance whose role has too
//     many or too few callables.
//   - [ParamRange] at a directive whose int param is below the minimum of
//     its spec.
//   - [ExclusiveParams] at a directive that writes two params that its spec
//     excludes from each other.
//   - [UnsharedParam] at a directive whose host-param reference resolves to
//     a parameter that the callable of another param does not declare.
//
// The validation of the directives reports the other faults of an
// instance, such as an unknown variant or a missing role.
//
// # Dependency position
//
// The package imports the packages shape and detectors, and the SDK
// facade's root, diag, directive, meta, node, plugin, position, rules and
// symbol packages.
package catalog
