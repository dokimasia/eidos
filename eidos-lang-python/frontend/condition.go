// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
)

// The names that the checks of conditions read: the module sys, its
// version and its platform, the method that tests the prefix of a text, the
// constant of typing that a type checker reads as true, and the operator
// and of a boolean expression.
const (
	sysModule    = "sys"
	versionInfo  = "version_info"
	platformName = "platform"
	startsWith   = "startswith"
	typeChecking = "TYPE_CHECKING"
	andOperator  = "and"
)

// The indexes of the major and the minor version of sys.version_info as
// written, and the major version of every version that the frontend reads.
const (
	majorIndex = "0"
	minorIndex = "1"
	major      = 3
)

// The operators of a comparison of a version or a platform.
const (
	lessThan     = "<"
	lessEqual    = "<="
	greaterThan  = ">"
	greaterEqual = ">="
	equal        = "=="
	notEqual     = "!="
)

// truth is the value of the check of a condition: true, false, or unknown
// for a condition that the frontend does not evaluate.
type truth uint8

// The values of a check.
const (
	unknown truth = iota
	isTrue
	isFalse
)

// ordered returns the value of a comparison of the version of the options
// with a version constant, as Python compares tuples, over the elements of
// the constant. An element of the constant past the version that the
// options state, such as a micro version, is unknown, and so is the
// comparison where that element would decide it.
func ordered(version, stated []int, operator string) truth {
	order := 0
	for i, want := range stated {
		if i >= len(version) {
			return unknown
		}
		if version[i] != want {
			order = version[i] - want
			break
		}
	}
	var met bool
	switch operator {
	case lessThan:
		met = order < 0
	case lessEqual:
		met = order <= 0
	case greaterThan:
		met = order > 0
	case greaterEqual:
		met = order >= 0
	case equal:
		met = order == 0
	case notEqual:
		met = order != 0
	default:
		return unknown
	}
	if met {
		return isTrue
	}
	return isFalse
}

// branches lowers the branches of an if statement that the checks of its
// conditions select, in order, through lowerBlock. A branch whose check is
// false, and every branch after a check that is true, does not load, and
// the file is stamped with the check that excluded it, as written. Every
// branch of a condition that the frontend does not evaluate loads.
func (l *lowering) branches(stmt treesitter.Node, lowerBlock func(treesitter.Node)) {
	decided := ""
	clause := func(condition, block treesitter.Node) {
		switch {
		case decided != "":
			l.exclude(decided)
		case condition.IsZero():
			lowerBlock(block)
		default:
			switch l.check(condition) {
			case isTrue:
				decided = condition.Compact()
				lowerBlock(block)
			case isFalse:
				l.exclude(condition.Compact())
			default:
				lowerBlock(block)
			}
		}
	}
	clause(stmt.Child(l.v.fieldCondition), stmt.Child(l.v.fieldConsequence))
	for alt := range stmt.Children(l.v.fieldAlternative) {
		if alt.Kind() == l.v.elifClause {
			clause(alt.Child(l.v.fieldCondition), alt.Child(l.v.fieldConsequence))
			continue
		}
		clause(treesitter.Node{}, alt.Child(l.v.fieldBody))
	}
}

// exclude records the check of a condition that excluded a branch in the
// stamp python.condition, once.
func (l *lowering) exclude(check string) {
	if !slices.Contains(l.conditions, check) {
		l.conditions = append(l.conditions, check)
	}
}

// check returns the value of a condition, as a type checker reads it:
//
//   - TYPE_CHECKING is true, as a name and as a member of typing.
//   - A comparison of sys.version_info with a tuple of integers, and of an
//     element or a slice of it with an integer or a tuple, compares with
//     the version of the options, the major version 3 and its minor
//     version.
//   - A comparison of sys.platform with a string by == or !=, and
//     sys.platform.startswith of a string, compare with the platform of the
//     options.
//   - not, and, or and parentheses combine checks.
//
// Every other condition is unknown.
func (l *lowering) check(n treesitter.Node) truth {
	switch n.Kind() {
	case l.v.parenthesizedExpression:
		return l.check(firstNamed(n))
	case l.v.notOperator:
		switch l.check(n.Child(l.v.fieldArgument)) {
		case isTrue:
			return isFalse
		case isFalse:
			return isTrue
		default:
			return unknown
		}
	case l.v.booleanOperator:
		left, right := l.check(n.Child(l.v.fieldLeft)), l.check(n.Child(l.v.fieldRight))
		decisive, other := isTrue, isFalse
		if n.Child(l.v.fieldOperator).Text() == andOperator {
			decisive, other = isFalse, isTrue
		}
		switch {
		case left == decisive || right == decisive:
			return decisive
		case left == other && right == other:
			return other
		default:
			return unknown
		}
	case l.v.identifier, l.v.attribute:
		if _, name := l.scope.canonical(n.Compact()); name == typeChecking {
			return isTrue
		}
	case l.v.comparisonOperator:
		return l.compare(n)
	case l.v.call:
		return l.platformPrefix(n)
	}
	return unknown
}

// compare returns the value of one comparison of the version or the
// platform of sys with a constant, and unknown for a chain of comparisons.
func (l *lowering) compare(n treesitter.Node) truth {
	operands := slices.Collect(n.Content())
	if len(operands) != 2 {
		return unknown
	}
	operator := n.Child(l.v.fieldOperators).Text()
	subject, constant := operands[0], operands[1]
	if l.sysMember(subject, platformName) {
		value, quoted := l.stringValue(constant)
		switch {
		case !quoted || operator != equal && operator != notEqual:
			return unknown
		case (value == l.platform) == (operator == equal):
			return isTrue
		default:
			return isFalse
		}
	}
	version := []int{major, l.minor}
	switch {
	case l.sysMember(subject, versionInfo):
	case subject.Kind() == l.v.subscript && l.sysMember(subject.Child(l.v.fieldValue), versionInfo):
		index := subject.Child(l.v.fieldSubscript)
		switch {
		case index.Kind() == l.v.integer && index.Text() == majorIndex:
			version = version[:1]
		case index.Kind() == l.v.integer && index.Text() == minorIndex:
			version = version[1:]
		case index.Kind() != l.v.slice:
			return unknown
		}
	default:
		return unknown
	}
	stated, read := l.versionConstant(constant)
	if !read {
		return unknown
	}
	return ordered(version, stated, operator)
}

// versionConstant returns the integers of a version constant: a tuple of
// integers, or one integer. It reports false for any other constant.
func (l *lowering) versionConstant(n treesitter.Node) ([]int, bool) {
	elements := []treesitter.Node{n}
	if n.Kind() == l.v.tuple {
		elements = slices.Collect(n.Content())
	}
	out := make([]int, 0, len(elements))
	for _, e := range elements {
		if e.Kind() != l.v.integer {
			return nil, false
		}
		v, err := strconv.Atoi(e.Text())
		if err != nil {
			return nil, false
		}
		out = append(out, v)
	}
	return out, len(out) > 0
}

// platformPrefix returns the value of sys.platform.startswith of a string,
// which compares the prefix with the platform of the options.
func (l *lowering) platformPrefix(n treesitter.Node) truth {
	method := n.Child(l.v.fieldFunction)
	args := slices.Collect(n.Child(l.v.fieldArguments).Content())
	prefixed := method.Kind() == l.v.attribute && method.Child(l.v.fieldAttribute).Text() == startsWith
	if !prefixed || !l.sysMember(method.Child(l.v.fieldObject), platformName) || len(args) != 1 {
		return unknown
	}
	prefix, quoted := l.stringValue(args[0])
	switch {
	case !quoted:
		return unknown
	case strings.HasPrefix(l.platform, prefix):
		return isTrue
	default:
		return isFalse
	}
}

// sysMember reports whether a name means a member of the module sys, as
// sys.platform after import sys, and platform after from sys import
// platform.
func (l *lowering) sysMember(n treesitter.Node, member string) bool {
	if k := n.Kind(); k != l.v.identifier && k != l.v.attribute {
		return false
	}
	module, name := l.scope.canonical(n.Compact())
	return module == sysModule && name == member
}
