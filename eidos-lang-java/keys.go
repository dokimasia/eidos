// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace every java key registers under.
const namespace = "java"

// The satellite's classification keys, which the frontend stamps.
const (
	// TestFileKey classifies a file under a src/test/ directory, the
	// layout Maven and Gradle share, or named as Surefire's default
	// includes name a test: Test*.java, *Test.java, *Tests.java and
	// *TestCase.java.
	TestFileKey meta.KeyName = "java.testFile"

	// RecordKey marks the struct a record declaration declares.
	RecordKey meta.KeyName = "java.record"

	// AnnotationTypeKey marks the interface an annotation type
	// declaration declares.
	AnnotationTypeKey meta.KeyName = "java.annotationType"

	// ModuleKey records, on a module-info.java file, the name of the
	// module it declares.
	ModuleKey meta.KeyName = "java.module"
)

// Keys claims the java namespace and registers every java key, in the
// form a composition and a corpus fixture declare them. It returns
// the claim's error, or every registration's error joined.
//
// # Allocation contract
//
// Keys allocates the three kind lists of the keys, and the registry
// allocates its namespace claim and the growth of its lists and maps to
// four keys: 11 allocations into a fresh registry.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	file := []symbol.Kind{symbol.KindFile}
	_, testErr := meta.Register[bool](r, meta.KeySpec{
		Name: TestFileKey, Kinds: file,
		Doc: "marks a file the Maven and Gradle layout or Surefire's includes name a test",
	})
	_, recordErr := meta.Register[bool](r, meta.KeySpec{
		Name: RecordKey, Kinds: []symbol.Kind{symbol.KindStruct},
		Doc: "marks the struct a record declaration declares",
	})
	_, annotationErr := meta.Register[bool](r, meta.KeySpec{
		Name: AnnotationTypeKey, Kinds: []symbol.Kind{symbol.KindInterface},
		Doc: "marks the interface an annotation type declaration declares",
	})
	_, moduleErr := meta.Register[string](r, meta.KeySpec{
		Name: ModuleKey, Kinds: file,
		Doc: "records the name of the module a module-info.java file declares",
	})
	return errors.Join(testErr, recordErr, annotationErr, moduleErr)
}
