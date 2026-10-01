// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"slices"
	"testing"
	"unicode/utf8"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// The constant pool tags no signature names, which the reader skips by
// their size, and one no class file version defines (§4.4).
const (
	tagFieldref           = 9
	tagMethodref          = 10
	tagInterfaceMethodref = 11
	tagMethodHandle       = 15
	tagDynamic            = 17
	tagInvokeDynamic      = 18
	tagModule             = 19
	tagPackage            = 20
	tagUnknown            = 2
)

// accentedName is a name of two-byte characters, which UTF-8 and
// modified UTF-8 encode alike.
const accentedName = "Größe"

// The descriptor of a method of one int parameter, and the bytes of a
// lone high surrogate in modified UTF-8.
const (
	oneIntMethod  = "(I)V"
	loneSurrogate = "\xed\xa0\x80"
)

// replacement is what an unpaired surrogate in a name decodes to.
const replacement = string(utf8.RuneError)

// The constant pool is what every item indexes, so how it is read and
// how an index into it is checked is pinned.
func TestPool(t *testing.T) {
	t.Parallel()

	t.Run("readPool", func(t *testing.T) {
		t.Parallel()

		t.Run("skips the constants no signature names by their size", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.constant(tagFieldref, 0, 1, 0, 2)
			b.constant(tagMethodref, 0, 1, 0, 2)
			b.constant(tagInterfaceMethodref, 0, 1, 0, 2)
			b.constant(tagNameAndType, 0, 1, 0, 2)
			b.constant(tagMethodHandle, 1, 0, 1)
			b.constant(tagMethodType, 0, 1)
			b.constant(tagDynamic, 0, 1, 0, 2)
			b.constant(tagInvokeDynamic, 0, 1, 0, 2)
			b.constant(tagModule, 0, 1)
			b.constant(tagPackage, 0, 1)
			b.constant(tagInteger, 0, 0, 0, 1)
			b.constant(tagFloat, 0, 0, 0, 1)
			b.constant(tagLong, 0, 0, 0, 0, 0, 0, 0, 1)
			b.constant(tagDouble, 0, 0, 0, 0, 0, 0, 0, 1)
			b.constant(tagString, 0, 1)
			assert.Equal(t, parsed(t, b).Name, fixtureName, "the items after the pool still decode")
		})

		t.Run("returns ErrMalformed for a constant of an unknown tag", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.constant(tagUnknown, 0, 1)
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "tag 2 is undefined")
		})

		t.Run("returns ErrMalformed for a long at the pool's last index", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.constant(tagLong, 0, 0, 0, 0, 0, 0, 0, 1)
			b.next--
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "its second slot is past the pool")
		})
	})

	t.Run("entry", func(t *testing.T) {
		t.Parallel()

		t.Run("returns ErrMalformed for an index past the pool", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.this = b.next
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "no constant has that index")
		})

		t.Run("returns ErrMalformed for an index that names a long's second slot", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.this = b.constant(tagLong, 0, 0, 0, 0, 0, 0, 0, 1) + 1
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "the slot has no constant")
		})

		t.Run("returns ErrMalformed for an index of a constant of another tag", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.this = b.utf8(fixtureName)
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "this_class names a Class")
		})
	})

	t.Run("utf8", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a name of two-byte characters", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.this = b.classRef(accentedName)
			assert.Equal(t, parsed(t, b).Name, accentedName, "UTF-8 and modified UTF-8 agree on it")
		})

		t.Run("decodes an unpaired surrogate in a name as U+FFFD", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.this = b.classRef(loneSurrogate)
			assert.Equal(t, parsed(t, b).Name, replacement, "a high surrogate alone")
		})

		t.Run("returns ErrMalformed for a name that is not modified UTF-8", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.this = b.classRef("\x00")
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "a zero byte")
		})

		t.Run("returns ErrMalformed for a name with a lone continuation byte", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.this = b.classRef(nameA + "\x80")
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "0x80 continues no sequence")
		})
	})

	t.Run("optionalUtf8", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a parameter without a name as empty", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.methods = append(b.methods, b.member(classfile.AccPublic, methodLabel, oneIntMethod,
				b.attribute(attrMethodParameters, slices.Concat([]byte{1}, u2(0), u2(0))...)))
			assert.Equal(t, parsed(t, b).Methods[0].Parameters, []classfile.Parameter{{}}, "index 0 names nothing")
		})
	})
}
