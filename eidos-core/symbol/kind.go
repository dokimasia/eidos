// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol

// Kind discriminates the declaration kinds.
//
// Every consumer that switches on kind uses one set of names across
// both model sides, so a switch written against the node model
// reads the same against the emit model. The zero value is
// [KindInvalid], which no declaration answers.
//
// One constant exists per schema struct, in schema declaration
// order. The values belong to a build: nothing durable stores them,
// because the JSON codecs encode kind names and the sealed state
// carries its own format version.
type Kind uint8

// The declaration kinds, one per schema struct.
const (
	KindInvalid Kind = iota
	KindPackage
	KindFile
	KindImport
	KindExport
	KindBinding
	KindStruct
	KindInterface
	KindEnum
	KindEnumVariant
	KindSum
	KindSumVariant
	KindAlias
	KindConstraint
	KindFunction
	KindMethod
	KindField
	KindParam
	KindReturn
	KindVariable
	KindConstant
	KindTypeParam
	KindTypeRef
	KindEmbed
)
