// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"bytes"
	"context"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The fixtures the class-file cases lower, which javac compiled from the
// class-file reader's test sources: with -parameters, Box alone without
// it, and the JAR of the first set.
const (
	classesDir       = "classfile/testdata/classes"
	plainDir         = "classfile/testdata/plain"
	libJAR           = "classfile/testdata/lib.jar"
	libDir           = "com/acme/lib"
	libPackage       = "com/acme/lib"
	libName          = "lib"
	classSuffix      = ".class"
	boxClass         = "Box"
	boxPath          = "com/acme/lib/Box.class"
	innerClass       = "Box$Inner"
	nestedClass      = "Box$Nested"
	callbackClass    = "Box$Callback"
	callbackPath     = "com/acme/lib/Box$Callback.class"
	localClass       = "Box$1Local"
	packageInfoClass = "package-info"
	jarMember        = "lib/lib.jar"
	looseFixture     = "classfile/testdata/unnamed/Loose.class"
	looseMember      = "Loose.class"
)

// The fixture classes the cases name besides Box.
const (
	baseClass   = "Base"
	colorClass  = "Color"
	figureClass = "Figure"
	hiddenClass = "Hidden"
	holderClass = "Holder"
	pointClass  = "Point"
	rangeClass  = "Range"
)

// The attribute name the cases rename in a class file's bytes, and the
// name of the same length it takes, which the reader skips.
const (
	methodParametersAttr = "MethodParameters"
	skippedAttr          = "MethodParameterz"
)

// The byte strings the crafted cases replace in a compiled fixture, each
// with one of the same length that javac does not write, in place of a
// class file another compiler wrote: all's descriptor with a last
// parameter that is no array, two's signature with more parameters than
// its descriptor, inner's signature naming a local class, any's
// signature naming a class of the unnamed package, Inner's constructor
// descriptor with a first parameter of another class, and the plain
// Inner constructor's MethodParameters entries, the first final and
// mandated, made final and synthetic.
const (
	allDescriptor    = "([Ljava/lang/String;)V"
	allTwoParams     = "(ILjava/lang/String;)V"
	twoSignature     = "<V:Ljava/lang/Object;>(TV;)V"
	twoLonger        = "<V:Ljava/lang/Object;>(III)V"
	innerSignature   = "()Lcom/acme/lib/Box<TT;>.Inner;"
	innerToLocal     = "()Lcom/acme/lib/Box$1Local<**>;"
	anySignature     = "Ljava/util/List<*>;"
	anyUnnamed       = "LLooseNamedType<*>;"
	innerDescriptor  = "(Lcom/acme/lib/Box;Ljava/lang/String;)V"
	innerOtherFirst  = "(Lcom/acme/lib/Bax;Ljava/lang/String;)V"
	mandatedEntries  = "\x02\x00\x00\x80\x10\x00\x00\x00\x00"
	syntheticEntries = "\x02\x00\x00\x10\x10\x00\x00\x00\x00"
)

// The names and spellings the class-file cases expect: the classes the
// fixtures reference, as a class file spells them, and their packages.
const (
	listSpelling       = "java.util.List"
	innerSpelling      = "com.acme.lib.Box$Inner"
	doneSpelling       = "com.acme.lib.Box$Callback$Done"
	localSpelling      = "com.acme.lib.Box$1Local"
	boxSpelling        = "com.acme.lib.Box"
	baxSpelling        = "com.acme.lib.Bax"
	dotSpelling        = "com.acme.lib.Dot"
	stringSpelling     = "java.lang.String"
	objectSpelling     = "java.lang.Object"
	numberSpelling     = "java.lang.Number"
	comparableSpelling = "java.lang.Comparable"
	markerName         = "com.acme.lib.Marker"
	deprecatedName     = "java.lang.Deprecated"
	utilPackage        = "java/util"
	langPackage        = "java/lang"
	ioPackage          = "java/io"
)

// badClassFileCode is the spelling of the code a class file that does
// not decode reports under, which findings persist.
const badClassFileCode = "JAVA-0006"

// A class file lowers to the declarations its source declares, so each
// kind's shape, what a signature-only load keeps, and the scope a class
// file's references resolve through, is pinned against javac's output.
func TestClasses(t *testing.T) {
	t.Parallel()

	t.Run("lowerClasses", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a class into the package its binary name names", func(t *testing.T) {
			t.Parallel()

			named[*node.Struct](t, libDecls(t), boxClass)
		})

		t.Run("annotates the File node of package-info with the package's annotations", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, packageInfoClass)
			assert.Equal(t, fileIn(t, gb, libPackage).Annotations,
				symbol.Annotations{{Name: markerName, Args: []string{`value = "pkg"`}}}, "the package's Marker")
		})

		t.Run("declares nothing for a local class", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthFull, localClass)
			assert.Empty(t, fileIn(t, gb, libPackage).Decls, "a method declares Local, whatever the depth keeps")
		})

		t.Run("declares nothing for a class signature depth leaves out", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, hiddenClass)
			assert.Empty(t, fileIn(t, gb, libPackage).Decls, "Hidden has package access")
		})

		t.Run("lowers a member class into the class that declares it", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, typeNames(box.Types), []string{"Callback", "Inner", "Nested"},
				"in name order, without Secret, which has package access")
		})

		t.Run("lowers member classes in name order whatever the members' order", func(t *testing.T) {
			t.Parallel()

			names := []string{boxClass, nestedClass, innerClass, callbackClass}
			tree := classTree(t, classesDir, names...)
			var paths []string
			for _, n := range names {
				paths = append(paths, path.Join(libDir, n+classSuffix))
			}
			gb, _ := parseInOrder(t, tree, plugin.DepthSignatures, paths...)
			box := named[*node.Struct](t, packageDecls(t, gb, libPackage), boxClass)
			assert.Equal(t, typeNames(box.Types), []string{"Callback", "Inner", "Nested"}, "Nested's file came first")
		})

		t.Run("imports the packages a class's declarations reference", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, boxClass)
			assert.Equal(t, importPaths(fileIn(t, gb, libPackage)), []string{ioPackage, langPackage, utilPackage},
				"IOException, Comparable and List among them, in path order")
		})

		t.Run("imports each package on demand", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, boxClass)
			for _, imp := range fileIn(t, gb, libPackage).Imports {
				assert.True(t, imp.Wildcard, "a class file names no class an import could bind")
			}
		})

		t.Run("names a package after its last path element", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, boxClass)
			assert.Equal(t, packageIn(t, gb, libPackage).Name, libName, "com/acme/lib is lib")
		})

		t.Run("declares a class of the unnamed package into the unnamed package", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{looseMember: {Data: mustRead(t, looseFixture)}}
			gb, _ := parseMembers(t, tree, plugin.DepthSignatures)
			named[*node.Struct](t, fileIn(t, gb, "").Decls, "Loose")
		})

		t.Run("names the unnamed package nothing", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{looseMember: {Data: mustRead(t, looseFixture)}}
			gb, _ := parseMembers(t, tree, plugin.DepthSignatures)
			assert.Equal(t, packageIn(t, gb, "").Name, "", "Java's unnamed package")
		})

		t.Run("declares nothing for a module-info class", func(t *testing.T) {
			t.Parallel()

			gb, _ := jarUnit(t, mustRead(t, libJAR), plugin.DepthSignatures)
			for _, p := range gb.Packages() {
				assert.NotEqual(t, p.ID.Package, "", "module-info opens no unnamed package")
			}
		})

		t.Run("positions a class at the start of its class file", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, box.Pos, position.Pos{File: boxPath, Line: 1, Col: 1}, "a class file has no lines")
		})
	})

	t.Run("declare", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves out a class with package access at signature depth", func(t *testing.T) {
			t.Parallel()

			hidden := func(s symbol.Symbol) bool { return nameOf(s) == hiddenClass }
			assert.False(t, slices.ContainsFunc(libDecls(t), hidden), "Hidden is not public")
		})

		t.Run("keeps a class with package access at full depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthFull, hiddenClass)
			named[*node.Struct](t, fileIn(t, gb, libPackage).Decls, hiddenClass)
		})
	})

	t.Run("class", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a class's superclass with its type arguments as what it extends", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, spellingsOf(box.Extends), []string{"com.acme.lib.Base"}, "Base")
			assert.Equal(t, box.Extends[0].Args[0].Spelling, "T", "of T")
		})

		t.Run("leaves out Object as what a class extends", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, named[*node.Struct](t, libDecls(t), baseClass).Extends, "source writes no extends Object")
		})

		t.Run("lowers a class's interfaces as what it implements", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, spellingsOf(box.Implements), []string{"com.acme.lib.Holder", "java.lang.Cloneable"}, "both")
		})

		t.Run("lowers the interfaces of a class without a signature as what it implements", func(t *testing.T) {
			t.Parallel()

			circle := named[*node.Struct](t, libDecls(t), "Circle")
			assert.Equal(t, spellingsOf(circle.Implements), []string{"com.acme.lib.Shape"},
				"from the class file's names")
		})

		t.Run("lowers an abstract class as abstract", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Struct](t, libDecls(t), baseClass).Abstract, "Base is abstract")
		})

		t.Run("lowers a final class as final", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Struct](t, libDecls(t), "Circle").Final, "Circle is final")
		})

		t.Run("lowers a sealed class as sealed", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Struct](t, libDecls(t), figureClass).Sealed, "Figure is sealed")
		})

		t.Run("lowers a sealed class's permitted subclasses", func(t *testing.T) {
			t.Parallel()

			figure := named[*node.Struct](t, libDecls(t), figureClass)
			assert.Equal(t, spellingsOf(figure.Permits), []string{dotSpelling}, "Dot")
		})

		t.Run("lowers a class's type parameters with their bounds", func(t *testing.T) {
			t.Parallel()

			tp := named[*node.Struct](t, libDecls(t), boxClass).TypeParams[0]
			assert.Equal(t, tp.Name, "T", "T")
			assert.Equal(t, spellingsOf(tp.Bounds), []string{comparableSpelling}, "extends Comparable<T>")
		})

		t.Run("lowers a type parameter bounded by nothing without bounds", func(t *testing.T) {
			t.Parallel()

			base := named[*node.Struct](t, libDecls(t), baseClass)
			assert.Empty(t, base.TypeParams[0].Bounds, "the Object bound is left out")
		})

		t.Run("lowers a static member class at the type level", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, named[*node.Struct](t, box.Types, "Nested").Level, symbol.LevelType, "Nested is static")
		})

		t.Run("lowers an inner class at the instance level", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, named[*node.Struct](t, box.Types, "Inner").Level, symbol.LevelInstance, "an inner class")
		})

		t.Run("annotates a class with its annotations", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, box.Annotations, symbol.Annotations{{Name: markerName, Args: []string{`value = "box"`}}},
				"the Marker with its pair")
		})

		t.Run("lowers a member class at the visibility its InnerClasses entry states", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.Equal(t, named[*node.Interface](t, box.Types, "Callback").Visibility, symbol.VisibilityProtected,
				"Callback is protected")
		})
	})

	t.Run("iface", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an interface with its type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, named[*node.Interface](t, libDecls(t), holderClass).TypeParams, 1, "T")
		})

		t.Run("lowers a sealed interface's permitted subtypes", func(t *testing.T) {
			t.Parallel()

			shape := named[*node.Interface](t, libDecls(t), "Shape")
			assert.True(t, shape.Sealed, "sealed")
			assert.Equal(t, spellingsOf(shape.Permits), []string{"com.acme.lib.Circle", "com.acme.lib.Square"}, "both")
		})

		t.Run("stamps an annotation interface java.annotationType", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, "Marker")
			assert.Equal(t, stampsOf(gb, java.AnnotationTypeKey), []any{true}, "Marker")
		})

		t.Run("stamps no other interface java.annotationType", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, holderClass)
			assert.Empty(t, stampsOf(gb, java.AnnotationTypeKey), "Holder is a plain interface")
		})

		t.Run("leaves out the Annotation an annotation interface extends", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, named[*node.Interface](t, libDecls(t), "Marker").Extends, "source writes no extends")
		})

		t.Run("lowers an interface's member types", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			callback := named[*node.Interface](t, box.Types, "Callback")
			assert.Equal(t, typeNames(callback.Types), []string{"Done"}, "Done is Callback's member")
		})
	})

	t.Run("enum", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an enum's constants as its variants", func(t *testing.T) {
			t.Parallel()

			color := named[*node.Enum](t, libDecls(t), colorClass)
			assert.Equal(t, variantNames(color.Variants), []string{"RED", "GREEN", "BLUE"}, "in declaration order")
		})

		t.Run("annotates an enum's constant with its annotations", func(t *testing.T) {
			t.Parallel()

			color := named[*node.Enum](t, libDecls(t), colorClass)
			assert.Equal(t, color.Variants[2].Annotations, symbol.Annotations{{Name: deprecatedName}}, "BLUE's")
		})

		t.Run("annotates an enum with its annotations", func(t *testing.T) {
			t.Parallel()

			color := named[*node.Enum](t, libDecls(t), colorClass)
			assert.Equal(t, color.Annotations,
				symbol.Annotations{{Name: markerName, Args: []string{`value = "color"`}}}, "Color's Marker")
		})

		t.Run("leaves an enum's constants out of its fields", func(t *testing.T) {
			t.Parallel()

			color := named[*node.Enum](t, libDecls(t), colorClass)
			assert.Equal(t, fieldNames(color.Fields), []string{"FIRST"}, "FIRST, and no constant")
		})

		t.Run("lowers an enum's methods with values and valueOf", func(t *testing.T) {
			t.Parallel()

			color := named[*node.Enum](t, libDecls(t), colorClass)
			assert.Equal(t, methodNames(color.Methods), []string{"values", "valueOf", "get"},
				"the two every enum declares implicitly, and its own")
		})

		t.Run("lowers an enum's valueOf with its mandated parameter", func(t *testing.T) {
			t.Parallel()

			color := named[*node.Enum](t, libDecls(t), colorClass)
			assert.Equal(t, paramNames(methodOf(t, color.Methods, "valueOf").Params), []string{"name"},
				"source declares it implicitly")
		})

		t.Run("leaves out an enum's private constructor", func(t *testing.T) {
			t.Parallel()

			color := named[*node.Enum](t, libDecls(t), colorClass)
			assert.False(t, slices.ContainsFunc(color.Methods, func(m *node.Method) bool { return m.Constructs }),
				"an enum's constructor is private")
		})
	})

	t.Run("record", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a record's components before its fields", func(t *testing.T) {
			t.Parallel()

			point := named[*node.Struct](t, libDecls(t), pointClass)
			assert.Equal(t, fieldNames(point.Fields), []string{"x", "y", "tags", "ORIGIN"},
				"the private fields behind the components are left out")
		})

		t.Run("lowers a component as public and immutable", func(t *testing.T) {
			t.Parallel()

			x := named[*node.Struct](t, libDecls(t), pointClass).Fields[0]
			assert.Equal(t, [2]any{x.Visibility, x.Mutability},
				[2]any{symbol.VisibilityPublic, symbol.MutabilityImmutable}, "an accessor's component")
		})

		t.Run("lowers a component's type arguments", func(t *testing.T) {
			t.Parallel()

			tags := named[*node.Struct](t, libDecls(t), pointClass).Fields[2]
			assert.Equal(t, spellingsOf(tags.Type.Args), []string{stringSpelling}, "List<String>")
		})

		t.Run("annotates a component with its annotations", func(t *testing.T) {
			t.Parallel()

			y := named[*node.Struct](t, libDecls(t), pointClass).Fields[1]
			assert.Equal(t, y.Annotations, symbol.Annotations{{Name: markerName, Args: []string{`value = "y"`}}}, "y's")
		})

		t.Run("lowers a record as final", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Struct](t, libDecls(t), pointClass).Final, "a record is final")
		})

		t.Run("lowers a record's interfaces as what it implements", func(t *testing.T) {
			t.Parallel()

			point := named[*node.Struct](t, libDecls(t), pointClass)
			assert.Equal(t, spellingsOf(point.Implements), []string{comparableSpelling}, "Comparable<Point>")
		})

		t.Run("lowers a record's type parameters", func(t *testing.T) {
			t.Parallel()

			pair := named[*node.Struct](t, named[*node.Struct](t, libDecls(t), "Outer").Types, "Pair")
			assert.Equal(t, pair.TypeParams[0].Name, "L", "Pair<L>")
		})

		t.Run("stamps a record java.record", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, pointClass)
			assert.Equal(t, stampsOf(gb, java.RecordKey), []any{true}, "Point")
		})

		t.Run("lowers a member record at the type level", func(t *testing.T) {
			t.Parallel()

			outer := named[*node.Struct](t, libDecls(t), "Outer")
			assert.Equal(t, named[*node.Struct](t, outer.Types, "Pair").Level, symbol.LevelType, "a record is static")
		})

		t.Run("lowers a record's member types", func(t *testing.T) {
			t.Parallel()

			point := named[*node.Struct](t, libDecls(t), pointClass)
			assert.Equal(t, typeNames(point.Types), []string{"Axis"}, "Axis is Point's member")
		})

		t.Run("lowers a compact constructor with the components as its named parameters", func(t *testing.T) {
			t.Parallel()

			point := named[*node.Struct](t, libDecls(t), pointClass)
			ctor := methodOf(t, point.Methods, pointClass)
			assert.Equal(t, paramNames(ctor.Params), []string{"x", "y", "tags"}, "mandated, and declared")
		})

		t.Run("lowers a compact constructor of a record without a signature with every component", func(t *testing.T) {
			t.Parallel()

			rng := named[*node.Struct](t, libDecls(t), rangeClass)
			assert.Equal(t, paramNames(methodOf(t, rng.Methods, rangeClass).Params), []string{"lo", "hi"},
				"MethodParameters flags both mandated, and lo is no enclosing instance")
		})
	})

	t.Run("fields", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a constant's value", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxField(t, "LIMIT").Value, "16", "LIMIT")
		})

		t.Run("lowers a static final field at the type level as immutable", func(t *testing.T) {
			t.Parallel()

			f := boxField(t, "LIMIT")
			want := [2]any{symbol.LevelType, symbol.MutabilityImmutable}
			assert.Equal(t, [2]any{f.Level, f.Mutability}, want, "a constant")
		})

		t.Run("lowers an instance field at the instance level as mutable", func(t *testing.T) {
			t.Parallel()

			f := boxField(t, "index")
			assert.Equal(t, [2]any{f.Level, f.Mutability}, [2]any{symbol.LevelInstance, symbol.MutabilityMutable},
				"an instance's own")
		})

		t.Run("lowers an interface's field at the type level as immutable", func(t *testing.T) {
			t.Parallel()

			f := named[*node.Interface](t, libDecls(t), holderClass).Fields[0]
			assert.Equal(t, [2]any{f.Level, f.Mutability}, [2]any{symbol.LevelType, symbol.MutabilityImmutable},
				"CONSTANT")
		})

		t.Run("annotates a field with its annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxField(t, "numbers").Annotations,
				symbol.Annotations{{Name: markerName, Args: []string{`value = "n"`}}}, "numbers' Marker")
		})

		t.Run("leaves out a field with package access at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.False(t, slices.Contains(fieldNames(named[*node.Struct](t, libDecls(t), boxClass).Fields), "hidden"),
				"hidden has package access")
		})

		t.Run("keeps a field with package access at full depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthFull, boxClass)
			box := named[*node.Struct](t, fileIn(t, gb, libPackage).Decls, boxClass)
			assert.True(t, slices.Contains(fieldNames(box.Fields), "hidden"), "every field but a private one")
		})

		t.Run("leaves out a private field at full depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthFull, boxClass)
			box := named[*node.Struct](t, fileIn(t, gb, libPackage).Decls, boxClass)
			assert.False(t, slices.Contains(fieldNames(box.Fields), "secret"), "no caller can name it")
		})

		t.Run("leaves out a synthetic field at full depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthFull, boxClass, innerClass)
			box := named[*node.Struct](t, packageDecls(t, gb, libPackage), boxClass)
			assert.Empty(t, named[*node.Struct](t, box.Types, "Inner").Fields, "this$0 has package access")
		})

		t.Run("lowers a field's type arguments", func(t *testing.T) {
			t.Parallel()

			arg := boxField(t, "numbers").Type.Args[0]
			assert.Equal(t, [2]any{arg.Form, arg.Variance}, [2]any{symbol.FormWildcard, symbol.VarianceOut},
				"? extends Number")
			assert.Equal(t, arg.Spelling, "?extends java.lang.Number", "as the compact form spells it")
		})

		t.Run("lowers a wildcard's bound as its child", func(t *testing.T) {
			t.Parallel()

			arg := boxField(t, "numbers").Type.Args[0]
			assert.Equal(t, spellingsOf(arg.Elems), []string{numberSpelling}, "Number")
		})

		t.Run("lowers an unbounded wildcard as a Wildcard without a child", func(t *testing.T) {
			t.Parallel()

			arg := boxField(t, "any").Type.Args[0]
			assert.Equal(t, [2]any{arg.Form, len(arg.Elems)}, [2]any{symbol.FormWildcard, 0}, "?")
		})
	})

	t.Run("methods", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a constructor as a method that constructs, named after its class", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			assert.True(t, methodOf(t, box.Methods, boxClass).Constructs, "Box(T value)")
		})

		t.Run("leaves out a bridge method", func(t *testing.T) {
			t.Parallel()

			box := named[*node.Struct](t, libDecls(t), boxClass)
			gets := slices.DeleteFunc(slices.Clone(box.Methods), func(m *node.Method) bool { return m.Name != "get" })
			assert.Length(t, gets, 1, "the get returning T alone")
		})

		t.Run("leaves out a class initializer at full depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthFull, colorClass)
			color := named[*node.Enum](t, fileIn(t, gb, libPackage).Decls, colorClass)
			assert.False(t, slices.Contains(methodNames(color.Methods), "<clinit>"), "no source declares it")
		})

		t.Run("annotates a method with its annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxMethod(t, "done").Annotations, symbol.Annotations{{Name: deprecatedName}}, "@Deprecated")
		})

		t.Run("lowers a method's parameter names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, paramNames(boxMethod(t, "done").Params), []string{"why", "code"}, "from MethodParameters")
		})

		t.Run("annotates a parameter with its annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxMethod(t, "done").Params[0].Annotations, symbol.Annotations{{Name: markerName}}, "why's")
		})

		t.Run("lowers a variadic method's last parameter as positionally variadic", func(t *testing.T) {
			t.Parallel()

			more := boxMethod(t, "conv").Params[1]
			assert.Equal(t, more.Variadic, symbol.VariadicPositional, "int... more")
			assert.Equal(t, more.Type.Spelling, "int", "of the element")
		})

		t.Run("lowers a variadic method's only parameter as positionally variadic", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxMethod(t, "all").Params[0].Variadic, symbol.VariadicPositional, "String... names")
		})

		t.Run("lowers the array parameter of a method that is not variadic as a List", func(t *testing.T) {
			t.Parallel()

			values := boxMethod(t, "fill").Params[0]
			assert.Equal(t, [2]any{values.Variadic, values.Type.Spelling}, [2]any{symbol.VariadicNone, "int[]"},
				"int[] values")
		})

		t.Run("lowers a method's type parameters", func(t *testing.T) {
			t.Parallel()

			tp := boxMethod(t, "conv").TypeParams[0]
			assert.Equal(t, [2]any{tp.Name, spellingsOf(tp.Bounds)}, [2]any{"U", []string{"T"}}, "U extends T")
		})

		t.Run("keeps both bounds of a type parameter bounded by Object and an interface", func(t *testing.T) {
			t.Parallel()

			tp := boxMethod(t, "max").TypeParams[0]
			assert.Equal(t, spellingsOf(tp.Bounds), []string{objectSpelling, comparableSpelling},
				"E extends Object & Comparable<? super E>")
		})

		t.Run("lowers the exceptions a method throws", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(boxMethod(t, "conv").Throws), []string{"java.io.IOException"}, "IOException")
		})

		t.Run("lowers a type variable a method throws", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(boxMethod(t, "fail").Throws), []string{"E"}, "throws E")
		})

		t.Run("lowers a void method without a result", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, boxMethod(t, "done").Returns, "void")
		})

		t.Run("lowers a static method at the type level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxMethod(t, "fail").Level, symbol.LevelType, "fail is static")
		})

		t.Run("lowers a final method as final", func(t *testing.T) {
			t.Parallel()

			assert.True(t, boxMethod(t, "done").Final, "done is final")
		})

		t.Run("lowers a class's method without a default", func(t *testing.T) {
			t.Parallel()

			assert.False(t, boxMethod(t, "done").HasDefault, "only an interface's method has one")
		})

		t.Run("lowers an interface's default method as having a default", func(t *testing.T) {
			t.Parallel()

			holder := named[*node.Interface](t, libDecls(t), holderClass)
			assert.True(t, methodOf(t, holder.Methods, "present").HasDefault, "present has a body")
		})

		t.Run("lowers an interface's abstract method as abstract", func(t *testing.T) {
			t.Parallel()

			holder := named[*node.Interface](t, libDecls(t), holderClass)
			assert.True(t, methodOf(t, holder.Methods, "get").Abstract, "get has none")
		})

		t.Run("lowers an interface's abstract method without a default", func(t *testing.T) {
			t.Parallel()

			holder := named[*node.Interface](t, libDecls(t), holderClass)
			assert.False(t, methodOf(t, holder.Methods, "get").HasDefault, "get has no body")
		})

		t.Run("lowers an interface's static method without a default", func(t *testing.T) {
			t.Parallel()

			holder := named[*node.Interface](t, libDecls(t), holderClass)
			assert.False(t, methodOf(t, holder.Methods, "empty").HasDefault, "no implementation inherits it")
		})

		t.Run("leaves out an interface's private method", func(t *testing.T) {
			t.Parallel()

			holder := named[*node.Interface](t, libDecls(t), holderClass)
			assert.False(t, slices.Contains(methodNames(holder.Methods), "helper"), "helper is private")
		})

		t.Run("lowers a method's array result as a List", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxMethod(t, "grid").Returns[0].Type.Spelling, "int[][]", "int[][]")
		})

		t.Run("lowers a super wildcard with Variance In", func(t *testing.T) {
			t.Parallel()

			arg := boxMethod(t, "conv").Params[0].Type.Args[0]
			assert.Equal(t, [2]any{arg.Variance, arg.Spelling}, [2]any{symbol.VarianceIn, "?super U"}, "? super U")
		})

		t.Run("lowers a member class reference with the type arguments of its last name", func(t *testing.T) {
			t.Parallel()

			ref := boxMethod(t, "inner").Returns[0].Type
			assert.Equal(t, [2]any{ref.Spelling, len(ref.Args)}, [2]any{innerSpelling, 0},
				"Box<T>.Inner: T is Box's, and Inner states none")
		})
	})

	t.Run("params", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an inner class constructor without its enclosing instance", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, paramNames(innerConstructor(t, classTree(t, classesDir, boxClass, innerClass)).Params),
				[]string{"label"}, "MethodParameters flags the instance mandated")
		})

		t.Run("lowers an inner class constructor of a class without parameter names without its enclosing instance",
			func(t *testing.T) {
				t.Parallel()

				ctor := innerConstructor(t, classTree(t, plainDir, boxClass, innerClass))
				assert.Equal(t, spellingsOf(paramTypes(ctor.Params)), []string{stringSpelling},
					"the enclosing Box is the descriptor's first parameter")
			})

		t.Run("lowers an inner class constructor's descriptor without its enclosing instance", func(t *testing.T) {
			t.Parallel()

			tree := classTree(t, plainDir, boxClass, innerClass)
			patch(t, tree, innerClass, methodParametersAttr, skippedAttr)
			assert.Equal(t, spellingsOf(paramTypes(innerConstructor(t, tree).Params)), []string{stringSpelling},
				"without MethodParameters, the descriptor's first parameter is the enclosing Box")
		})

		t.Run("lowers an inner class constructor without the parameter MethodParameters flags mandated",
			func(t *testing.T) {
				t.Parallel()

				tree := classTree(t, classesDir, boxClass, innerClass)
				patch(t, tree, innerClass, innerDescriptor, innerOtherFirst)
				assert.Equal(t, spellingsOf(paramTypes(innerConstructor(t, tree).Params)), []string{stringSpelling},
					"MethodParameters decides, whatever the first parameter's type")
			})

		t.Run("lowers an inner class constructor without the parameter MethodParameters flags synthetic",
			func(t *testing.T) {
				t.Parallel()

				tree := classTree(t, plainDir, boxClass, innerClass)
				patch(t, tree, innerClass, mandatedEntries, syntheticEntries)
				assert.Equal(t, spellingsOf(paramTypes(innerConstructor(t, tree).Params)), []string{stringSpelling},
					"source does not declare a synthetic parameter")
			})

		t.Run("keeps an inner class constructor's first parameter of another class without MethodParameters",
			func(t *testing.T) {
				t.Parallel()

				tree := classTree(t, plainDir, boxClass, innerClass)
				patch(t, tree, innerClass, methodParametersAttr, skippedAttr)
				patch(t, tree, innerClass, innerDescriptor, innerOtherFirst)
				assert.Equal(t, spellingsOf(paramTypes(innerConstructor(t, tree).Params)),
					[]string{baxSpelling, stringSpelling}, "only the enclosing class's instance is implicit")
			})

		t.Run("keeps a static member class constructor's first parameter of the outer class", func(t *testing.T) {
			t.Parallel()

			gb, _ := parseMembers(t, classTree(t, plainDir, boxClass, nestedClass), plugin.DepthSignatures)
			box := named[*node.Struct](t, packageDecls(t, gb, libPackage), boxClass)
			ctor := methodOf(t, named[*node.Struct](t, box.Types, "Nested").Methods, "Nested")
			assert.Equal(t, spellingsOf(paramTypes(ctor.Params)), []string{boxSpelling},
				"Nested has no enclosing instance")
		})

		t.Run("keeps an inner class method's first parameter of the outer class", func(t *testing.T) {
			t.Parallel()

			gb, _ := parseMembers(t, classTree(t, plainDir, boxClass, innerClass), plugin.DepthSignatures)
			box := named[*node.Struct](t, packageDecls(t, gb, libPackage), boxClass)
			attach := methodOf(t, named[*node.Struct](t, box.Types, "Inner").Methods, "attach")
			assert.Equal(t, spellingsOf(paramTypes(attach.Params)), []string{boxSpelling},
				"only a constructor's is implicit")
		})

		t.Run("lowers a method's descriptor where its signature states more parameters", func(t *testing.T) {
			t.Parallel()

			tree := classTree(t, classesDir, boxClass)
			patch(t, tree, boxClass, twoSignature, twoLonger)
			gb, _ := parseMembers(t, tree, plugin.DepthSignatures)
			box := named[*node.Struct](t, packageDecls(t, gb, libPackage), boxClass)
			assert.Equal(t, spellingsOf(paramTypes(methodOf(t, box.Methods, "two").Params)), []string{objectSpelling},
				"the signature does not describe the method")
		})

		t.Run("lowers a varargs method's last parameter that is no array as not variadic", func(t *testing.T) {
			t.Parallel()

			tree := classTree(t, classesDir, boxClass)
			patch(t, tree, boxClass, allDescriptor, allTwoParams)
			gb, _ := parseMembers(t, tree, plugin.DepthSignatures)
			box := named[*node.Struct](t, packageDecls(t, gb, libPackage), boxClass)
			assert.Equal(t, methodOf(t, box.Methods, "all").Params[1].Variadic, symbol.VariadicNone,
				"a String collects no arguments")
		})

		t.Run("names no parameter of a class without parameter names", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, plainDir, plugin.DepthSignatures, boxClass)
			box := named[*node.Struct](t, fileIn(t, gb, libPackage).Decls, boxClass)
			assert.Equal(t, paramNames(methodOf(t, box.Methods, "done").Params), []string{"", ""}, "positional")
		})
	})

	t.Run("ref", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a class by its binary name in dotted form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxField(t, "numbers").Type.Spelling, listSpelling, "java.util.List")
		})

		t.Run("records a class's package as the reference's package", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boxField(t, "numbers").Type.Package, utilPackage, "java/util")
		})

		t.Run("records a member class's identity in the File node's scope", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, boxClass)
			assert.Equal(t, resolved(fileScope(t, gb, boxPath), innerSpelling),
				plugin.Candidates{{nested(libPackage, boxClass, "Inner")}}, "Box$Inner is Box's Inner")
		})

		t.Run("records the identity of a member class of a member class in the File node's scope", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, boxClass, callbackClass)
			assert.Equal(t, resolved(fileScope(t, gb, callbackPath), doneSpelling),
				plugin.Candidates{{nested(libPackage, "Box.Callback", "Done")}}, "Done is Box's Callback's Done")
		})

		t.Run("records a top-level class's identity in the File node's scope", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, boxClass)
			assert.Equal(t, resolved(fileScope(t, gb, boxPath), listSpelling),
				plugin.Candidates{{id(utilPackage, "List")}}, "one candidate")
		})

		t.Run("records a local class's identity as a top-level class's", func(t *testing.T) {
			t.Parallel()

			tree := classTree(t, classesDir, boxClass)
			patch(t, tree, boxClass, innerSignature, innerToLocal)
			gb, _ := parseMembers(t, tree, plugin.DepthSignatures)
			assert.Equal(t, resolved(fileScope(t, gb, boxPath), localSpelling),
				plugin.Candidates{{id(libPackage, localClass)}}, "Local's entry names no declaring class")
		})

		t.Run("resolves nothing for a spelling the class file does not name", func(t *testing.T) {
			t.Parallel()

			gb, _ := classUnit(t, classesDir, plugin.DepthSignatures, boxClass)
			assert.Empty(t, resolved(fileScope(t, gb, boxPath), "T"), "a type variable names no class")
		})

		t.Run("imports no package for a class of the unnamed package", func(t *testing.T) {
			t.Parallel()

			tree := classTree(t, classesDir, boxClass)
			patch(t, tree, boxClass, anySignature, anyUnnamed)
			gb, _ := parseMembers(t, tree, plugin.DepthSignatures)
			assert.Equal(t, importPaths(fileIn(t, gb, libPackage)), []string{ioPackage, langPackage, utilPackage},
				"no import names the unnamed package")
		})
	})

	t.Run("decodeClass", func(t *testing.T) {
		t.Parallel()

		t.Run("reports BadClassFile for a class file that does not decode", func(t *testing.T) {
			t.Parallel()

			_, found := parseMembers(t, fstest.MapFS{"bad.class": {Data: []byte("junk")}}, plugin.DepthSignatures)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadClassFile}, "the magic number is wrong")
		})

		t.Run("reports a class file that does not decode as JAVA-0006", func(t *testing.T) {
			t.Parallel()

			_, found := parseMembers(t, fstest.MapFS{"bad.class": {Data: []byte("junk")}}, plugin.DepthSignatures)
			assert.NotEmpty(t, found, "the class file reports")
			assert.Equal(t, found[0].Code.String(), badClassFileCode,
				"under the satellite's prefix and the sixth number")
		})

		t.Run("returns the context's error for a class file of a done context", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, cancelledParse(t, looseMember, mustRead(t, looseFixture)), context.Canceled,
				"a cancelled load decodes nothing")
		})

		t.Run("returns the context's error for a JAR of a done context", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, cancelledParse(t, jarMember, mustRead(t, libJAR)), context.Canceled,
				"a cancelled load opens no JAR")
		})
	})
}

// libDecls returns the declarations of the fixture library's classes, a
// unit of every one of them loaded at signature depth.
func libDecls(tb assert.TB) node.Symbols {
	tb.Helper()

	entries, err := os.ReadDir(path.Join(classesDir, libDir))
	assert.NoError(tb, err, "the fixtures are on disk")
	var names []string
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), classSuffix))
	}
	gb, _ := classUnit(tb, classesDir, plugin.DepthSignatures, names...)
	var out node.Symbols
	for _, f := range packageIn(tb, gb, libPackage).Files {
		out = append(out, f.Decls...)
	}
	return out
}

// classTree returns a tree of fixture class files of a directory by
// their simple names.
func classTree(tb assert.TB, dir string, names ...string) fstest.MapFS {
	tb.Helper()

	tree := fstest.MapFS{}
	for _, n := range names {
		file := path.Join(libDir, n+classSuffix)
		tree[file] = &fstest.MapFile{Data: mustRead(tb, path.Join(dir, file))}
	}
	return tree
}

// classUnit parses a unit whose members are fixture class files of a
// directory by their simple names, at a depth.
func classUnit(tb assert.TB, dir string, depth plugin.Depth, names ...string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parseMembers(tb, classTree(tb, dir, names...), depth)
}

// patch replaces the one occurrence of a byte string in a tree's class
// file of a simple name with another of the same length.
func patch(tb assert.TB, tree fstest.MapFS, name, old, replacement string) {
	tb.Helper()

	f := tree[path.Join(libDir, name+classSuffix)]
	assert.Equal(tb, bytes.Count(f.Data, []byte(old)), 1, "the class file has the bytes once")
	f.Data = bytes.Replace(f.Data, []byte(old), []byte(replacement), 1)
}

// jarUnit parses a unit whose one member is a JAR of the given bytes, at
// a depth.
func jarUnit(tb assert.TB, data []byte, depth plugin.Depth) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parseMembers(tb, fstest.MapFS{jarMember: {Data: data}}, depth)
}

// parseMembers parses a unit whose members are every file of a tree, in
// path order, at a depth.
func parseMembers(tb assert.TB, tree fstest.MapFS, depth plugin.Depth) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parseInOrder(tb, tree, depth, slices.Sorted(maps.Keys(tree))...)
}

// parseInOrder parses a unit whose members are files of a tree in the
// order given, at a depth.
func parseInOrder(
	tb assert.TB, tree fstest.MapFS, depth plugin.Depth, paths ...string,
) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	f := frontend.New(nil)
	refs := make([]plugin.SourceRef, 0, len(paths))
	for _, p := range paths {
		refs = append(refs, plugin.SourceRef{Path: p})
	}
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(refs, tree, depth, f.Syntax(), brand, sink, f.Name())
	assert.NoError(tb, f.Parse(context.Background(), u), "the unit parses")
	return u.Graph(), slices.Collect(sink.All())
}

// cancelledParse parses a unit of one member of some bytes under a
// context that is done, and returns the parse's error.
func cancelledParse(tb assert.TB, member string, data []byte) error {
	tb.Helper()

	f := frontend.New(nil)
	u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: member}}, fstest.MapFS{member: {Data: data}},
		plugin.DepthSignatures, f.Syntax(), brand, diag.NewSink(), f.Name())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return f.Parse(ctx, u)
}

// mustRead returns a fixture file's bytes.
func mustRead(tb assert.TB, p string) []byte {
	tb.Helper()

	data, err := os.ReadFile(p)
	assert.NoError(tb, err, "the fixture is on disk")
	return data
}

// packageDecls returns the declarations of every File node of a
// builder's package of a path.
func packageDecls(tb assert.TB, gb *plugin.GraphBuilder, pkg string) node.Symbols {
	tb.Helper()

	var out node.Symbols
	for _, f := range packageIn(tb, gb, pkg).Files {
		out = append(out, f.Decls...)
	}
	return out
}

// boxField returns the fixture library's Box field of a name.
func boxField(tb assert.TB, name string) *node.Field {
	tb.Helper()

	fields := named[*node.Struct](tb, libDecls(tb), boxClass).Fields
	i := slices.IndexFunc(fields, func(f *node.Field) bool { return f.Name == name })
	assert.True(tb, i >= 0, "Box declares the field")
	return fields[i]
}

// boxMethod returns the fixture library's Box method of a name.
func boxMethod(tb assert.TB, name string) *node.Method {
	tb.Helper()

	return methodOf(tb, named[*node.Struct](tb, libDecls(tb), boxClass).Methods, name)
}

// methodOf returns the first method of a name among methods.
func methodOf(tb assert.TB, methods []*node.Method, name string) *node.Method {
	tb.Helper()

	i := slices.IndexFunc(methods, func(m *node.Method) bool { return m.Name == name })
	assert.True(tb, i >= 0, "the type declares the method")
	return methods[i]
}

// innerConstructor returns the constructor of Box's Inner, from a tree
// of Box's and Inner's class files.
func innerConstructor(tb assert.TB, tree fstest.MapFS) *node.Method {
	tb.Helper()

	gb, _ := parseMembers(tb, tree, plugin.DepthSignatures)
	box := named[*node.Struct](tb, packageDecls(tb, gb, libPackage), boxClass)
	return methodOf(tb, named[*node.Struct](tb, box.Types, "Inner").Methods, "Inner")
}

// fileScope returns the scope the File node of a member path recorded.
func fileScope(tb assert.TB, gb *plugin.GraphBuilder, p string) plugin.ImportScope {
	tb.Helper()

	for _, rec := range gb.Scopes() {
		if rec.File.Path == p {
			return plugin.ImportScope{Bindings: rec.Bindings}
		}
	}
	tb.Fatalf("no File node of %s recorded a scope", p)
	return plugin.ImportScope{}
}

// typeNames returns the names of nested declarations, in order.
func typeNames(types node.Symbols) []string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, nameOf(t))
	}
	return out
}

// importPaths returns the paths of a File node's imports, in order.
func importPaths(f *node.File) []string {
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		out = append(out, imp.Path)
	}
	return out
}

// paramTypes returns the types of parameters, in order.
func paramTypes(params []*node.Param) []*node.TypeRef {
	out := make([]*node.TypeRef, 0, len(params))
	for _, p := range params {
		out = append(out, p.Type)
	}
	return out
}
