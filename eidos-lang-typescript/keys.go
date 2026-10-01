// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace every typescript key registers
// under.
const namespace = "typescript"

// The satellite's classification keys, which the frontend stamps.
const (
	// TestFileKey classifies a file Jest's default match names a test:
	// one under a __tests__ directory, one named test or spec, and one
	// whose name ends in .test or .spec before its extension.
	TestFileKey meta.KeyName = "typescript.testFile"

	// NamespaceKey records, on the package a namespace declares, the
	// namespace's dotted name as the file spells it, such as A.B.
	NamespaceKey meta.KeyName = "typescript.namespace"

	// CallSignatureKey records the call signatures of an interface or
	// of the object type an alias names, each as written, because the
	// model has no member for a callable object.
	CallSignatureKey meta.KeyName = "typescript.callSignature"
)

// Keys claims the typescript namespace and registers every typescript
// key, in the shape a composition and a corpus fixture declare them.
// It returns the claim's error, or every registration's error joined.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	_, testErr := meta.Register[bool](r, meta.KeySpec{
		Name: TestFileKey, Kinds: []symbol.Kind{symbol.KindFile},
		Doc: "marks a file Jest's default match names a test",
	})
	_, namespaceErr := meta.Register[string](r, meta.KeySpec{
		Name: NamespaceKey, Kinds: []symbol.Kind{symbol.KindPackage},
		Doc: "records the dotted name of the namespace a package declares",
	})
	_, callErr := meta.Register[[]string](r, meta.KeySpec{
		Name: CallSignatureKey, Kinds: []symbol.Kind{symbol.KindInterface, symbol.KindAlias},
		Doc: "records the call signatures of a callable object type as written",
	})
	return errors.Join(testErr, namespaceErr, callErr)
}
