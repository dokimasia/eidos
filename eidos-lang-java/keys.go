// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace that every java key registers under.
const namespace = "java"

// The qualified names of the annotations that mark a type as the output of
// a foreign generator.
const (
	// GeneratedAnnotation is the annotation of the module java.compiler,
	// which the JDK declares from release 9.
	GeneratedAnnotation = "javax.annotation.processing.Generated"

	// LegacyGeneratedAnnotation is the annotation of JSR 250, which the
	// JDK declares in releases 8 to 10 and which generators for those
	// releases write.
	LegacyGeneratedAnnotation = "javax.annotation.Generated"
)

// The classification keys of the satellite, which the frontend stamps.
const (
	// TestFileKey classifies a file under a src/test/ directory, the
	// layout that Maven and Gradle share, or a file whose name matches the
	// default includes of Surefire: Test*.java, *Test.java, *Tests.java
	// and *TestCase.java.
	TestFileKey meta.KeyName = "java.testFile"

	// GeneratedKey records, on a file with a top-level type that has
	// [GeneratedAnnotation] or [LegacyGeneratedAnnotation], the annotation
	// as written.
	GeneratedKey meta.KeyName = "java.generated"

	// RecordKey marks the struct that a record declaration declares.
	RecordKey meta.KeyName = "java.record"

	// AnnotationTypeKey marks the interface that an annotation type
	// declaration declares.
	AnnotationTypeKey meta.KeyName = "java.annotationType"

	// ModuleKey records, on a module-info.java file, the name of the
	// module that the file declares.
	ModuleKey meta.KeyName = "java.module"
)

// Keys claims the java namespace and registers every java key, in the
// form that a composition and a corpus fixture declare them. It returns
// the error of the claim, or the errors of every registration joined.
//
// # Allocation contract
//
// Keys allocates the three kind lists of the keys, and the registry
// allocates its namespace claim and the growth of its lists and maps to
// five keys: 13 allocations into a fresh registry.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	file := []symbol.Kind{symbol.KindFile}
	_, testErr := meta.Register[bool](r, meta.KeySpec{
		Name: TestFileKey, Kinds: file,
		Doc: "marks a file that the Maven and Gradle layout or the includes of Surefire mark as a test",
	})
	_, generatedErr := meta.Register[string](r, meta.KeySpec{
		Name: GeneratedKey, Kinds: file,
		Doc: "records the annotation of a foreign generator on a top-level type of a file",
	})
	_, recordErr := meta.Register[bool](r, meta.KeySpec{
		Name: RecordKey, Kinds: []symbol.Kind{symbol.KindStruct},
		Doc: "marks the struct that a record declaration declares",
	})
	_, annotationErr := meta.Register[bool](r, meta.KeySpec{
		Name: AnnotationTypeKey, Kinds: []symbol.Kind{symbol.KindInterface},
		Doc: "marks the interface that an annotation type declaration declares",
	})
	_, moduleErr := meta.Register[string](r, meta.KeySpec{
		Name: ModuleKey, Kinds: file,
		Doc: "records the name of the module that a module-info.java file declares",
	})
	return errors.Join(testErr, generatedErr, recordErr, annotationErr, moduleErr)
}
