// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"math"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// A size in a config file has one of these two YAML tags.
const (
	intTag = "!!int"
	strTag = "!!str"
)

// sizePattern is the pattern of a size with a unit in the JSON Schema of a
// size: digits followed by the suffix of one of the units.
const sizePattern = "^[0-9]+(KiB|MiB|GiB)$"

// units contains the unit suffixes of a size and the number of bytes in
// each unit.
var units = [...]struct {
	suffix string
	size   int64
}{
	{suffix: "KiB", size: 1024},
	{suffix: "MiB", size: 1048576},
	{suffix: "GiB", size: 1073741824},
}

// Bytes is a size in bytes. In a config file, a size is a number, or a
// number followed by KiB, MiB or GiB, such as 512MiB.
type Bytes int64

var _ jsonschema.Schemer = Bytes(0)

// UnmarshalYAML decodes a size from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n when
// n is not a size or the size exceeds the largest int64.
func (b *Bytes) UnmarshalYAML(n *yaml.Node) error {
	refused := lineFault(n, "%q is not a size: use a number of bytes, or a number followed by KiB, MiB or GiB",
		n.Value)
	switch {
	case n.ShortTag() == intTag:
		var size int64
		if err := n.Decode(&size); err != nil || size < 0 {
			return refused
		}
		*b = Bytes(size)
		return nil
	case n.ShortTag() != strTag:
		return refused
	}
	for _, u := range units {
		digits, unit := strings.CutSuffix(n.Value, u.suffix)
		if !unit {
			continue
		}
		count, err := strconv.ParseUint(digits, 10, 63)
		if err != nil || count > uint64(math.MaxInt64/u.size) {
			return refused
		}
		*b = Bytes(int64(count) * u.size)
		return nil
	}
	return refused
}

// JSONSchema returns the JSON Schema of a size, a new map on each call. The
// schema allows a non-negative integer, and a string of digits followed by
// KiB, MiB or GiB.
func (Bytes) JSONSchema() map[string]any {
	return map[string]any{keywordOneOf: []any{
		map[string]any{keywordType: typeInteger, keywordMinimum: 0},
		map[string]any{keywordType: typeString, keywordPattern: sizePattern},
	}}
}
