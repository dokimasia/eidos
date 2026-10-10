// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// Modified UTF-8 differs from UTF-8 in its zero and its supplementary
// characters, so each form it decodes, and each it refuses, is pinned
// through a string constant.
func TestMutf8(t *testing.T) {
	t.Parallel()

	t.Run("decodeMUTF8", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "decodes NUL from its two-byte form", give: "\xc0\x80", want: javaString(0)},
			{name: "decodes a two-byte character", give: "\xc3\xa9", want: javaString(0xe9)},
			{name: "decodes a three-byte character", give: "\xe2\x82\xac", want: javaString(0x20ac)},
			{
				name: "decodes a supplementary character from its two surrogates", give: "\xed\xa0\xbd\xed\xb8\x80",
				want: javaString(0xd83d, 0xde00),
			},
			{name: "decodes ASCII as itself", give: "ok", want: `"ok"`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := stringConstant(tt.give)
				assert.NoError(t, err, "the constant decodes")
				assert.Equal(t, got, tt.want, "each code unit spelled")
			})
		}

		bad := []struct {
			name string
			give string
		}{
			{name: "returns ErrMalformed for a zero byte", give: "a\x00"},
			{name: "returns ErrMalformed for a byte from 0xF0 up before two continuation bytes", give: "\xf0\x9f\x98"},
			{name: "returns ErrMalformed for a two-byte sequence that ends early", give: "\xc3"},
			{name: "returns ErrMalformed for a three-byte sequence that ends early", give: "\xe2\x82"},
			{name: "returns ErrMalformed for a two-byte sequence without its continuation", give: "\xc3A"},
			{name: "returns ErrMalformed for a three-byte sequence without its first continuation", give: "\xe2A\x82"},
			{name: "returns ErrMalformed for a three-byte sequence without its last continuation", give: "\xe2\x82A"},
			{name: "returns ErrMalformed for a lone continuation byte", give: "\x80"},
		}
		for _, tt := range bad {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := stringConstant(tt.give)
				assert.ErrorIs(t, err, classfile.ErrMalformed, "the bytes are not modified UTF-8")
			})
		}
	})
}

// stringConstant assembles a class with one String constant of raw bytes
// and returns its spelling and the parse's error.
func stringConstant(raw string) (string, error) {
	b := newClassBuilder()
	b.fields = append(b.fields, b.member(classfile.AccPublic|classfile.AccStatic, fieldLabel, stringDesc,
		b.attribute(attrConstantValue, u2(int(b.constant(tagString, u2(int(b.utf8(raw)))...)))...)))
	c, err := classfile.Parse(b.bytes())
	if err != nil {
		return "", err
	}
	return c.Fields[0].Value, nil
}
