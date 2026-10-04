// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"encoding/binary"
	"errors"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// The fixtures javac 27 compiled once with --release 25 from the
// sources under testdata/src: with -parameters into testdata/classes,
// and Box alone without it into testdata/plain.
const (
	classesDir = "testdata/classes/com/acme/lib"
	plainDir   = "testdata/plain/com/acme/lib"
	classExt   = ".class"
	release25  = 69
)

// The fixtures' file names, without the extension.
const (
	boxFile         = "Box"
	innerFile       = "Box$Inner"
	pointFile       = "Point"
	shapeFile       = "Shape"
	packageInfoFile = "package-info"
	speccedFile     = "Specced"
)

// The binary names and descriptors the fixtures declare and reference.
const (
	boxName      = "com/acme/lib/Box"
	baseName     = "com/acme/lib/Base"
	holderName   = "com/acme/lib/Holder"
	markerName   = "com/acme/lib/Marker"
	circleName   = "com/acme/lib/Circle"
	squareName   = "com/acme/lib/Square"
	cloneable    = "java/lang/Cloneable"
	objectName   = "java/lang/Object"
	ioException  = "java/io/IOException"
	deprecated   = "java/lang/Deprecated"
	stringName   = "java/lang/String"
	initName     = "<init>"
	objectDesc   = "Ljava/lang/Object;"
	stringDesc   = "Ljava/lang/String;"
	intDesc      = "I"
	voidMethod   = "()V"
	twoIntMethod = "(II)V"
)

// The class, members and annotations the assembler writes: the class's
// name, a field's, a method's and a component's, and two annotation
// interfaces by descriptor and by binary name.
const (
	fixtureName    = "Fixture"
	fieldLabel     = "f"
	methodLabel    = "m"
	componentLabel = "x"
	annotationA    = "LA;"
	annotationB    = "LB;"
	nameA          = "A"
	nameB          = "B"
)

// shortAttribute is the fault the reader reports for an attribute of two
// bytes with one byte left.
const shortAttribute = "1 bytes are left where 2 are read"

// parseAllocs is one pass over the fixture library's 27 class files,
// 1,767 in every run with the collector on and off, about 65 a class
// file: the strings each constant pool decodes, the types each
// descriptor and signature parses into, and the lists of each class's
// members, attributes and annotations.
const parseAllocs = 1_767

// The attribute names the assembler writes, which pin the names the
// reader decodes (§4.7).
const (
	attrSignature        = "Signature"
	attrConstantValue    = "ConstantValue"
	attrRecord           = "Record"
	attrPermitted        = "PermittedSubclasses"
	attrMethodParameters = "MethodParameters"
	attrVisible          = "RuntimeVisibleAnnotations"
	attrInvisible        = "RuntimeInvisibleAnnotations"
	attrVisibleParams    = "RuntimeVisibleParameterAnnotations"
	attrInvisibleParams  = "RuntimeInvisibleParameterAnnotations"
	attrCustom           = "Custom"
)

// The constant pool tags and the magic number the assembler writes,
// which pin the format the reader decodes (§4.1, §4.4).
const (
	tagUtf8        = 1
	tagInteger     = 3
	tagFloat       = 4
	tagLong        = 5
	tagDouble      = 6
	tagClass       = 7
	tagString      = 8
	tagNameAndType = 12
	tagMethodType  = 16
	classMagic     = 0xCAFEBABE
)

// classBuilder assembles a class file for the cases the compiled
// fixtures do not state: a malformed item, and a constant javac does not
// write. Its class is named Fixture, public, and extends
// java/lang/Object.
type classBuilder struct {
	pool    []byte
	next    uint16
	this    uint16
	super   uint16
	fields  [][]byte
	methods [][]byte
	attrs   [][]byte
	trailer []byte
}

// newClassBuilder returns a builder of an empty class Fixture.
func newClassBuilder() *classBuilder {
	b := &classBuilder{next: 1}
	b.this = b.classRef(fixtureName)
	b.super = b.classRef(objectName)
	return b
}

// constant adds a constant of a tag and its payload and returns its
// index. A long or a double takes two indexes.
func (b *classBuilder) constant(tag byte, payload ...byte) uint16 {
	i := b.next
	b.pool = append(append(b.pool, tag), payload...)
	b.next++
	if tag == tagLong || tag == tagDouble {
		b.next++
	}
	return i
}

// utf8 adds a Utf8 constant of raw bytes, modified UTF-8 or not, and
// returns its index.
func (b *classBuilder) utf8(raw string) uint16 {
	return b.constant(tagUtf8, append(u2(len(raw)), raw...)...)
}

// classRef adds a Class constant naming a binary name and returns its
// index.
func (b *classBuilder) classRef(name string) uint16 {
	return b.constant(tagClass, u2(int(b.utf8(name)))...)
}

// attribute returns an attribute of a name and its body.
func (b *classBuilder) attribute(name string, body ...byte) []byte {
	return slices.Concat(u2(int(b.utf8(name))), u4(len(body)), body)
}

// marker returns the body of an annotations attribute of one annotation
// of a descriptor without elements.
func (b *classBuilder) marker(desc string) []byte {
	return slices.Concat(u2(1), u2(int(b.utf8(desc))), u2(0))
}

// member returns a field_info or a method_info structure.
func (b *classBuilder) member(access classfile.Access, name, desc string, attrs ...[]byte) []byte {
	return slices.Concat(u2(int(access)), u2(int(b.utf8(name))), u2(int(b.utf8(desc))), u2(len(attrs)),
		slices.Concat(attrs...))
}

// bytes returns the class file.
func (b *classBuilder) bytes() []byte {
	return slices.Concat(u4(classMagic), u2(0), u2(release25), u2(int(b.next)), b.pool,
		u2(int(classfile.AccPublic|classfile.AccSuper)), u2(int(b.this)), u2(int(b.super)), u2(0),
		u2(len(b.fields)), slices.Concat(b.fields...), u2(len(b.methods)), slices.Concat(b.methods...),
		u2(len(b.attrs)), slices.Concat(b.attrs...), b.trailer)
}

// A class file decodes into its class, its members and the attributes a
// signature reads, so the decoding of each structure is pinned against
// what javac wrote.
func TestClass(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes the version javac wrote for release 25", func(t *testing.T) {
			t.Parallel()

			c := fixture(t, boxFile)
			assert.Equal(t, [2]uint16{c.Major, c.Minor}, [2]uint16{release25, 0}, "major and minor")
		})

		t.Run("decodes a class's binary name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, boxFile).Name, boxName, "the internal form")
		})

		t.Run("decodes a class's access flags", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, boxFile).Access, classfile.AccPublic|classfile.AccSuper, "public")
		})

		t.Run("decodes a class's superclass", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, boxFile).Super, baseName, "Base")
		})

		t.Run("decodes no superclass for a class that names none", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.super = 0
			assert.Equal(t, parsed(t, b).Super, "", "a zero index names no class, as java/lang/Object's does")
		})

		t.Run("decodes a class's interfaces in order", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, boxFile).Interfaces, []string{holderName, cloneable}, "both")
		})

		t.Run("decodes every field of a class including the private ones", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNames(fixture(t, boxFile)), []string{
				"LIMIT", "BIG", "RATIO", "HALF", "NAN", "LETTER", "ON", "SMALL", "MID", "NAME", "numbers", "index",
				"any", "hidden", "secret",
			}, "in source order")
		})

		t.Run("decodes a bridge method with its flags", func(t *testing.T) {
			t.Parallel()

			bridge := methodNamed(t, fixture(t, boxFile), "get", objectName)
			assert.True(t, bridge.Access.Has(classfile.AccBridge|classfile.AccSynthetic), "javac's bridge")
		})

		t.Run("decodes a record's components", func(t *testing.T) {
			t.Parallel()

			c := fixture(t, pointFile)
			assert.True(t, c.Record, "Point is a record")
			assert.Equal(t, componentNames(c), []string{"x", "y", "tags"}, "in order")
		})

		t.Run("reports no class that is not a record as one", func(t *testing.T) {
			t.Parallel()

			assert.False(t, fixture(t, boxFile).Record, "Box has no Record attribute")
		})

		t.Run("decodes the subclasses a sealed interface permits", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, shapeFile).Permitted, []string{circleName, squareName}, "both")
		})

		t.Run("decodes a class's run-time visible annotations before its invisible ones", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.attrs = append(b.attrs, b.attribute(attrInvisible, b.marker(annotationA)...),
				b.attribute(attrVisible, b.marker(annotationB)...))
			assert.Equal(t, parsed(t, b).Annotations, []classfile.Annotation{{Type: nameB}, {Type: nameA}},
				"whatever the attributes' order")
		})

		t.Run("decodes a package's annotations from package-info", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, packageInfoFile).Annotations, []classfile.Annotation{{
				Type: markerName, Elements: []classfile.Element{{Name: "value", Value: `"pkg"`}},
			}}, "the package's Marker")
		})

		t.Run("returns ErrMalformed for a wrong magic number", func(t *testing.T) {
			t.Parallel()

			data := fixtureBytes(t, classesDir, boxFile)
			data[0] = 0
			_, err := classfile.Parse(data)
			assert.ErrorIs(t, err, classfile.ErrMalformed, "no class file opens so")
		})

		t.Run("returns ErrMalformed for bytes after the attributes", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.trailer = []byte{0}
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "a class file ends with its attributes")
		})

		t.Run("returns no class for a malformed class file", func(t *testing.T) {
			t.Parallel()

			c, _ := classfile.Parse([]byte{0xCA})
			assert.True(t, c == nil, "nothing half-decoded")
		})
	})

	t.Run("attributes", func(t *testing.T) {
		t.Parallel()

		t.Run("skips an attribute the reader does not decode by its length", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.attrs = append(b.attrs, b.attribute(attrCustom, 0xff, 0xff, 0xff))
			_, err := classfile.Parse(b.bytes())
			assert.NoError(t, err, "the body is any bytes")
		})

		t.Run("returns ErrMalformed for an attribute with bytes past its items", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			body := slices.Concat(u2(int(b.utf8(objectDesc))), []byte{0})
			b.attrs = append(b.attrs, b.attribute(attrSignature, body...))
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "a Signature is two bytes")
		})

		t.Run("returns ErrMalformed for an attribute longer than the bytes left", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			attr := b.attribute(attrCustom, 1, 2)
			b.attrs = append(b.attrs, attr[:len(attr)-1])
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "the length is past the end")
		})

		t.Run("reports the length of a decoded attribute longer than the bytes left", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			attr := b.attribute(attrSignature, u2(int(b.utf8(objectDesc)))...)
			b.attrs = append(b.attrs, attr[:len(attr)-1])
			_, err := classfile.Parse(b.bytes())
			assert.HasError(t, err, "the length is past the end")
			assert.True(t, strings.Contains(err.Error(), shortAttribute), "and not a fault of a body left unread")
		})

		t.Run("returns ErrMalformed for an attribute whose decode failed", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.attrs = append(b.attrs, b.attribute(attrPermitted, slices.Concat(u2(1), u2(int(b.utf8(nameA))))...))
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "a Utf8 is no Class")
		})
	})

	t.Run("field", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a field's descriptor type", func(t *testing.T) {
			t.Parallel()

			f := fieldNamed(t, fixture(t, boxFile), "hidden")
			assert.Equal(t, f.Type, classfile.Type{Kind: classfile.KindBase, Base: 'I'}, "int")
		})

		t.Run("decodes a field's signature", func(t *testing.T) {
			t.Parallel()

			f := fieldNamed(t, fixture(t, boxFile), "numbers")
			assert.Equal(t, f.Signature.Class[0].Args[0].Bound, classfile.BoundExtends, "List<? extends Number>")
		})

		t.Run("decodes no signature for a field whose type uses no type argument", func(t *testing.T) {
			t.Parallel()

			assert.True(t, fieldNamed(t, fixture(t, boxFile), "hidden").Signature == nil, "int")
		})

		t.Run("decodes a field's flags", func(t *testing.T) {
			t.Parallel()

			f := fieldNamed(t, fixture(t, boxFile), "LIMIT")
			assert.Equal(t, f.Access, classfile.AccPublic|classfile.AccStatic|classfile.AccFinal, "a constant")
		})

		t.Run("decodes a field's run-time visible annotations before its invisible ones", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			invisible := b.attribute(attrInvisible, b.marker(annotationA)...)
			visible := b.attribute(attrVisible, b.marker(annotationB)...)
			b.fields = append(b.fields, b.member(classfile.AccPublic, fieldLabel, intDesc, invisible, visible))
			assert.Equal(t, parsed(t, b).Fields[0].Annotations, []classfile.Annotation{{Type: nameB}, {Type: nameA}},
				"whatever the attributes' order")
		})

		t.Run("skips a field attribute the reader does not decode", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.fields = append(b.fields, b.member(classfile.AccPublic, fieldLabel, intDesc, b.attribute(attrCustom, 1)))
			assert.Length(t, parsed(t, b).Fields, 1, "the field decodes")
		})
	})

	t.Run("method", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a method's descriptor parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, methodNamed(t, fixture(t, boxFile), "done", "").Params, 2, "String and int")
		})

		t.Run("decodes a void method's result as nil", func(t *testing.T) {
			t.Parallel()

			assert.True(t, methodNamed(t, fixture(t, boxFile), "done", "").Result == nil, "void")
		})

		t.Run("decodes the exceptions a method declares", func(t *testing.T) {
			t.Parallel()

			m := methodNamed(t, fixture(t, boxFile), "conv", "")
			assert.Equal(t, m.Exceptions, []string{ioException}, "IOException")
		})

		t.Run("decodes a method's parameter names", func(t *testing.T) {
			t.Parallel()

			m := methodNamed(t, fixture(t, boxFile), "done", "")
			assert.Equal(t, m.Parameters, []classfile.Parameter{
				{Name: "why"}, {Name: "code", Access: classfile.AccFinal},
			}, "javac -parameters wrote them")
		})

		t.Run("decodes an inner class constructor's enclosing instance as mandated", func(t *testing.T) {
			t.Parallel()

			m := methodNamed(t, fixture(t, innerFile), initName, "")
			assert.True(t, m.Parameters[0].Access.Has(classfile.AccMandated), "the Box the Inner is in")
		})

		t.Run("decodes no parameter names for a class javac compiled without -parameters", func(t *testing.T) {
			t.Parallel()

			c := parsedFile(t, plainDir, boxFile)
			assert.True(t, methodNamed(t, c, "done", "").Parameters == nil, "no MethodParameters")
		})

		t.Run("decodes a method's parameter annotations", func(t *testing.T) {
			t.Parallel()

			m := methodNamed(t, fixture(t, boxFile), "done", "")
			assert.Equal(t, m.ParamAnnotations, [][]classfile.Annotation{{{Type: markerName}}, nil},
				"why's Marker, and none for code")
		})

		t.Run("decodes no parameter annotations for a method without either attribute", func(t *testing.T) {
			t.Parallel()

			conv := methodNamed(t, fixture(t, boxFile), "conv", "")
			assert.True(t, conv.ParamAnnotations == nil, "no parameter of conv is annotated")
		})

		t.Run("decodes a method's annotations", func(t *testing.T) {
			t.Parallel()

			m := methodNamed(t, fixture(t, boxFile), "done", "")
			assert.Equal(t, m.Annotations, []classfile.Annotation{{Type: deprecated}}, "@Deprecated")
		})

		t.Run("decodes a method's run-time visible annotations before its invisible ones", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			invisible := b.attribute(attrInvisible, b.marker(annotationA)...)
			visible := b.attribute(attrVisible, b.marker(annotationB)...)
			b.methods = append(b.methods, b.member(classfile.AccPublic, methodLabel, voidMethod, invisible, visible))
			assert.Equal(t, parsed(t, b).Methods[0].Annotations, []classfile.Annotation{{Type: nameB}, {Type: nameA}},
				"whatever the attributes' order")
		})

		t.Run("returns ErrMalformed for a method descriptor that does not parse", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.methods = append(b.methods, b.member(classfile.AccPublic, methodLabel, "(Q)V"))
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "Q is no type")
		})
	})

	t.Run("component", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a component's annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, pointFile).Components[1].Annotations, []classfile.Annotation{{
				Type: markerName, Elements: []classfile.Element{{Name: "value", Value: `"y"`}},
			}}, "y's Marker")
		})

		t.Run("decodes a component's signature", func(t *testing.T) {
			t.Parallel()

			tags := fixture(t, pointFile).Components[2]
			assert.Equal(t, tags.Signature.Class[0].Args[0].Type.BinaryName(), stringName, "List<String>")
		})

		t.Run("decodes a component's descriptor type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, pointFile).Components[0].Type.Base, byte('I'), "int x")
		})

		t.Run("decodes a component's run-time visible annotations before its invisible ones", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			component := slices.Concat(u2(int(b.utf8(componentLabel))), u2(int(b.utf8(intDesc))), u2(3),
				b.attribute(attrInvisible, b.marker(annotationA)...), b.attribute(attrCustom, 1),
				b.attribute(attrVisible, b.marker(annotationB)...))
			b.attrs = append(b.attrs, b.attribute(attrRecord, slices.Concat(u2(1), component)...))
			got := parsed(t, b).Components[0].Annotations
			assert.Equal(t, got, []classfile.Annotation{{Type: nameB}, {Type: nameA}},
				"whatever the attributes' order, and the unknown attribute skipped")
		})
	})

	t.Run("innerClass", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a nested class's declaring class", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, innerNamed(t, fixture(t, boxFile), "Callback").Outer, boxName, "Box declares Callback")
		})

		t.Run("decodes a nested class's own flags", func(t *testing.T) {
			t.Parallel()

			ic := innerNamed(t, fixture(t, boxFile), "Callback")
			assert.True(t, ic.Access.Has(classfile.AccProtected|classfile.AccStatic|classfile.AccInterface),
				"a protected member interface")
		})

		t.Run("decodes no declaring class for a local class", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, innerNamed(t, fixture(t, boxFile), "Local").Outer, "", "a method declares it")
		})

		t.Run("decodes no name for an anonymous class", func(t *testing.T) {
			t.Parallel()

			c := fixture(t, boxFile)
			i := slices.IndexFunc(c.Inner, func(ic classfile.InnerClass) bool { return ic.Inner == boxName+"$1" })
			assert.Equal(t, c.Inner[i].Name, "", "Box$1 has none")
		})
	})

	t.Run("constant", func(t *testing.T) {
		t.Parallel()

		t.Run("returns ErrMalformed for a constant on a field of a class other than String", func(t *testing.T) {
			t.Parallel()

			_, err := constantField(objectDesc, tagInteger, u4(1)...)
			assert.ErrorIs(t, err, classfile.ErrMalformed, "no constant initializes an Object")
		})

		wrong := []struct {
			name string
			desc string
			tag  byte
		}{
			{name: "returns ErrMalformed for an int field's constant of another tag", desc: intDesc, tag: tagFloat},
			{name: "returns ErrMalformed for a long field's constant of another tag", desc: "J", tag: tagInteger},
			{name: "returns ErrMalformed for a float field's constant of another tag", desc: "F", tag: tagInteger},
			{name: "returns ErrMalformed for a double field's constant of another tag", desc: "D", tag: tagInteger},
			{name: "returns ErrMalformed for a String constant of another tag", desc: stringDesc, tag: tagInteger},
		}
		for _, tt := range wrong {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := constantField(tt.desc, tt.tag, u4(1)...)
				assert.ErrorIs(t, err, classfile.ErrMalformed, "the constant's tag is the field type's")
			})
		}
	})

	t.Run("methodSignature", func(t *testing.T) {
		t.Parallel()

		t.Run("returns ErrMalformed for a method signature that does not parse", func(t *testing.T) {
			t.Parallel()

			_, err := signedMethod("()")
			assert.ErrorIs(t, err, classfile.ErrMalformed, "no result")
		})
	})

	t.Run("classSignature", func(t *testing.T) {
		t.Parallel()

		t.Run("returns ErrMalformed for a class signature that does not parse", func(t *testing.T) {
			t.Parallel()

			_, err := signedClass(intDesc)
			assert.ErrorIs(t, err, classfile.ErrMalformed, "a superclass is a class type")
		})
	})

	t.Run("alignedFromEnd", func(t *testing.T) {
		t.Parallel()

		t.Run("aligns a shorter list of parameter annotations to the last parameters", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.methods = append(b.methods, b.member(classfile.AccPublic, methodLabel, twoIntMethod,
				b.attribute(attrVisibleParams, slices.Concat([]byte{2}, u2(0), u2(0))...),
				b.attribute(attrInvisibleParams, slices.Concat([]byte{1}, b.marker(annotationA))...)))
			assert.Equal(t, parsed(t, b).Methods[0].ParamAnnotations, [][]classfile.Annotation{nil, {{Type: nameA}}},
				"the one invisible entry annotates the last parameter")
		})

		t.Run("aligns a shorter list of visible parameter annotations to the last parameters", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.methods = append(b.methods, b.member(classfile.AccPublic, methodLabel, twoIntMethod,
				b.attribute(attrVisibleParams, slices.Concat([]byte{1}, b.marker(annotationB))...),
				b.attribute(attrInvisibleParams, slices.Concat([]byte{2}, u2(0), u2(0))...)))
			assert.Equal(t, parsed(t, b).Methods[0].ParamAnnotations, [][]classfile.Annotation{nil, {{Type: nameB}}},
				"the one visible entry annotates the last parameter")
		})

		t.Run("lists a parameter's run-time visible annotations before its invisible ones", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.methods = append(b.methods, b.member(classfile.AccPublic, methodLabel, oneIntMethod,
				b.attribute(attrInvisibleParams, slices.Concat([]byte{1}, b.marker(annotationA))...),
				b.attribute(attrVisibleParams, slices.Concat([]byte{1}, b.marker(annotationB))...)))
			assert.Equal(t, parsed(t, b).Methods[0].ParamAnnotations,
				[][]classfile.Annotation{{{Type: nameB}, {Type: nameA}}}, "whatever the attributes' order")
		})
	})
}

// One pass of the reader over the fixture library allocates within
// parseAllocs in the ordinary run, which runs no benchmark. The check
// runs alone, because AllocsPerRun refuses to run beside parallel tests.
func TestClassAllocs(t *testing.T) {
	files := libraryFiles(t)
	var err error
	assert.MaxAllocs(t, func() {
		for _, data := range files {
			if _, err = classfile.Parse(data); err != nil {
				return
			}
		}
	}, parseAllocs, "Parse allocates within its ceiling over the fixture library")
	assert.NoError(t, err, "every fixture decodes")
}

// FuzzParse checks that no input panics the reader: every input decodes
// to a class or returns an error that wraps ErrMalformed, never both.
func FuzzParse(f *testing.F) {
	entries, err := os.ReadDir(classesDir)
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(path.Join(classesDir, e.Name()))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := classfile.Parse(data)
		if err != nil && !errors.Is(err, classfile.ErrMalformed) {
			t.Fatalf("the error %v wraps no ErrMalformed", err)
		}
		if (c == nil) == (err == nil) {
			t.Fatalf("the class %v and the error %v", c, err)
		}
	})
}

// BenchmarkClass measures the reader over every class file of the
// fixture library, which javac wrote, and fails above parseAllocs.
func BenchmarkClass(b *testing.B) {
	b.Run("Parse", func(b *testing.B) {
		files := libraryFiles(b)
		c := bench.Start(b).MaxAllocs(parseAllocs)
		defer c.End()
		for c.Loop() {
			for _, data := range files {
				if _, err := classfile.Parse(data); err != nil {
					b.Fatalf("the fixture decodes: %v", err)
				}
			}
		}
	})
}

// fixture parses one class file of testdata/classes by its file name.
func fixture(tb assert.TB, name string) *classfile.Class {
	tb.Helper()

	return parsedFile(tb, classesDir, name)
}

// parsedFile parses one class file of a fixture directory by its file
// name.
func parsedFile(tb assert.TB, dir, name string) *classfile.Class {
	tb.Helper()

	c, err := classfile.Parse(fixtureBytes(tb, dir, name))
	assert.NoError(tb, err, "the fixture decodes")
	return c
}

// libraryFiles returns every class file of the fixture library.
func libraryFiles(tb assert.TB) [][]byte {
	tb.Helper()

	entries, err := os.ReadDir(classesDir)
	assert.NoError(tb, err, "the fixture library is on disk")
	files := make([][]byte, 0, len(entries))
	for _, e := range entries {
		files = append(files, fixtureBytes(tb, classesDir, strings.TrimSuffix(e.Name(), classExt)))
	}
	return files
}

// fixtureBytes returns one class file of a fixture directory.
func fixtureBytes(tb assert.TB, dir, name string) []byte {
	tb.Helper()

	data, err := os.ReadFile(path.Join(dir, name+classExt))
	assert.NoError(tb, err, "the fixture is on disk")
	return data
}

// parsed parses an assembled class file.
func parsed(tb assert.TB, b *classBuilder) *classfile.Class {
	tb.Helper()

	c, err := classfile.Parse(b.bytes())
	assert.NoError(tb, err, "the assembled class decodes")
	return c
}

// constantField assembles a class with one static field of a descriptor
// whose ConstantValue is a constant of a tag and payload, and returns
// the field's value and the parse's error.
func constantField(desc string, tag byte, payload ...byte) (string, error) {
	b := newClassBuilder()
	b.fields = append(b.fields, b.member(classfile.AccPublic|classfile.AccStatic, fieldLabel, desc,
		b.attribute(attrConstantValue, u2(int(b.constant(tag, payload...)))...)))
	c, err := classfile.Parse(b.bytes())
	if err != nil {
		return "", err
	}
	return c.Fields[0].Value, nil
}

// fieldNames returns the names of a class's fields, in order.
func fieldNames(c *classfile.Class) []string {
	out := make([]string, 0, len(c.Fields))
	for _, f := range c.Fields {
		out = append(out, f.Name)
	}
	return out
}

// componentNames returns the names of a record's components, in order.
func componentNames(c *classfile.Class) []string {
	out := make([]string, 0, len(c.Components))
	for _, comp := range c.Components {
		out = append(out, comp.Name)
	}
	return out
}

// fieldNamed returns a class's field of a name.
func fieldNamed(tb assert.TB, c *classfile.Class, name string) classfile.Field {
	tb.Helper()

	i := slices.IndexFunc(c.Fields, func(f classfile.Field) bool { return f.Name == name })
	assert.True(tb, i >= 0, "the class declares the field")
	return c.Fields[i]
}

// methodNamed returns a class's method of a name, and of a result class
// where result is not empty, which tells a bridge from the method it
// bridges to.
func methodNamed(tb assert.TB, c *classfile.Class, name, result string) classfile.Method {
	tb.Helper()

	i := slices.IndexFunc(c.Methods, func(m classfile.Method) bool {
		return m.Name == name && (result == "" || m.Result != nil && m.Result.BinaryName() == result)
	})
	assert.True(tb, i >= 0, "the class declares the method")
	return c.Methods[i]
}

// innerNamed returns a class's InnerClasses entry of a simple name.
func innerNamed(tb assert.TB, c *classfile.Class, name string) classfile.InnerClass {
	tb.Helper()

	i := slices.IndexFunc(c.Inner, func(ic classfile.InnerClass) bool { return ic.Name == name })
	assert.True(tb, i >= 0, "the class lists the nested class")
	return c.Inner[i]
}

// u2 returns a value's two big-endian bytes.
func u2(v int) []byte { return binary.BigEndian.AppendUint16(nil, uint16(v)) }

// u4 returns a value's four big-endian bytes.
func u4(v int) []byte { return binary.BigEndian.AppendUint32(nil, uint32(v)) }
