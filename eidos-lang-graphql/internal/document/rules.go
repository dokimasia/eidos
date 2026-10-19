// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/validator/core"
	validatorrules "github.com/vektah/gqlparser/v2/validator/rules"
)

// LoneAnonymous is the message of an anonymous operation beside another
// operation of its document, as the validator of gqlparser words it. A
// client validates the documents of a project together, so it checks each
// document for the rule itself.
const LoneAnonymous = "This anonymous operation must be the only defined operation."

// repeatedName is the message of a name that two operations declare, as the
// rule of unique operation names of gqlparser words it.
const repeatedName = `There can be only one operation named "%s".`

// Rules returns the rules of the validation of the documents of a client:
// the default rules of gqlparser without the rules that graphql-tools leaves
// out for a client, which refuse a fragment that no operation spreads, a
// variable that no field reads and a directive that the schema does not
// define, such as a directive of a client cache.
//
// A client validates the documents of a project together, so a spread
// reaches a fragment of another document, and it checks each document for a
// lone anonymous operation itself. The set therefore leaves out the rule of
// a lone anonymous operation, and its rule of unique operation names reads
// the named operations alone, because the anonymous operations of two
// documents do not share a name. Rules builds a new set on every call for
// the caller to change, and allocates the set of gqlparser and the growth of
// its map.
func Rules() *validatorrules.Rules {
	rules := validatorrules.NewDefaultRules()
	for _, r := range [...]string{
		validatorrules.LoneAnonymousOperationRule.Name, validatorrules.NoUnusedFragmentsRule.Name,
		validatorrules.NoUnusedVariablesRule.Name, validatorrules.KnownDirectivesRule.Name,
	} {
		rules.RemoveRule(r)
	}
	rules.ReplaceRule(validatorrules.UniqueOperationNamesRule.Name, uniqueNamed)
	return rules
}

// uniqueNamed is the rule of unique operation names over the named
// operations of a document: it refuses each named operation whose name an
// earlier operation declares.
func uniqueNamed(observers *core.Events, addError core.AddErrFunc) {
	seen := map[string]bool{}
	observers.OnOperation(func(_ *core.Walker, operation *ast.OperationDefinition) {
		if operation.Name == "" {
			return
		}
		if seen[operation.Name] {
			addError(core.Message(repeatedName, operation.Name), core.At(operation.Position))
		}
		seen[operation.Name] = true
	})
}
