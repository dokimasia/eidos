// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import "go.dokimi.dev/eidos/sdk/directive"

// Namespace is the namespace of every key of the catalog. A key of a
// classification is the namespace, the name of the spec and a part,
// joined by dots, such as shape.writer.shape, and the group of a
// classification is the namespace and the name of the spec, such as
// shape.writer.
const Namespace = "shape"

// The parts of the keys of the catalog. A summary key is the namespace
// and its part, such as shape.classified. A family key is the namespace,
// the name of a spec and the part of its form, such as shape.atomic.mixin.
// A contract also has the key of its instance's id, such as shape.tx.id.
const (
	// PartDetected is the summary key of the detected shapes that report a
	// callable, in the order of precedence.
	PartDetected = "detected"
	// PartShape is the summary key of a callable's shape, and the part of
	// the family key of a shape.
	PartShape = "shape"
	// PartMixed is the summary key of a callable with a mixin.
	PartMixed = "mixed"
	// PartMember is the summary key of a callable in a contract instance.
	PartMember = "member"
	// PartClassified is the summary key of a callable with a
	// classification.
	PartClassified = "classified"
	// PartMixin is the part of the family key of a mixin.
	PartMixin = "mixin"
	// PartRole is the part of the family key of a contract, whose value is
	// the callable's role.
	PartRole = "role"
	// PartID is the part of the key of a contract instance's id.
	PartID = "id"
)

// Summary is one summary key of the catalog. Part is the part of the key,
// and Doc is the meaning of the key as a noun phrase.
type Summary struct {
	Part string
	Doc  string
}

// Summaries lists the summary keys of the catalog. A spec cannot have the
// part of one as its name, because the group of the spec would have the
// spelling of the summary key.
var Summaries = []Summary{
	{Part: PartDetected, Doc: "the detected shapes whose detectors report a callable, in the order of precedence"},
	{Part: PartShape, Doc: "the name of the shape of a callable"},
	{Part: PartMixed, Doc: "the mark of a callable with a mixin"},
	{Part: PartMember, Doc: "the mark of a callable with a role in a contract instance"},
	{Part: PartClassified, Doc: "the mark of a callable with a classification"},
}

// ReservedKeys lists the keys that no param and no binding of a spec can
// have. They are the routing keys of the directive layer, the role and
// the id of a contract instance, and the parts of the family keys of a
// shape and a mixin. The role is also the part of the family key of a
// contract.
var ReservedKeys = []string{
	string(directive.ReservedOut), string(directive.ReservedTag), PartRole, PartID, PartShape, PartMixin,
}
