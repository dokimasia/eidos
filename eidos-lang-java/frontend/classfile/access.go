// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

// Access is a set of access and property flags: a class's (§4.1), a
// field's (§4.5), a method's (§4.6), a nested class's (§4.7.6) or a
// parameter's (§4.7.24). One bit means different flags in different
// contexts, and each context reads the flags it defines.
type Access uint16

// The access and property flags.
const (
	// AccPublic is a class, member or nested class that is public.
	AccPublic Access = 0x0001
	// AccPrivate is a member or nested class that is private.
	AccPrivate Access = 0x0002
	// AccProtected is a member or nested class that is protected.
	AccProtected Access = 0x0004
	// AccStatic is a member or nested class that is static.
	AccStatic Access = 0x0008
	// AccFinal is a class, member, nested class or parameter that is
	// final.
	AccFinal Access = 0x0010
	// AccSuper is a class whose invokespecial follows the semantics
	// every class has had since Java SE 8.
	AccSuper Access = 0x0020
	// AccSynchronized is a method that is synchronized.
	AccSynchronized Access = 0x0020
	// AccVolatile is a field that is volatile.
	AccVolatile Access = 0x0040
	// AccBridge is a bridge method a compiler generated.
	AccBridge Access = 0x0040
	// AccTransient is a field that is transient.
	AccTransient Access = 0x0080
	// AccVarargs is a method whose last parameter is variadic.
	AccVarargs Access = 0x0080
	// AccNative is a method that is native.
	AccNative Access = 0x0100
	// AccInterface is a class or nested class that is an interface.
	AccInterface Access = 0x0200
	// AccAbstract is a class, method or nested class that is abstract.
	AccAbstract Access = 0x0400
	// AccStrict is a method whose floating-point mode is strict.
	AccStrict Access = 0x0800
	// AccSynthetic is a class, member, nested class or parameter the
	// source code does not declare.
	AccSynthetic Access = 0x1000
	// AccAnnotation is a class or nested class that is an annotation
	// interface.
	AccAnnotation Access = 0x2000
	// AccEnum is a class or nested class that is an enum, or a field
	// that is one of its constants.
	AccEnum Access = 0x4000
	// AccModule is a class file that declares a module.
	AccModule Access = 0x8000
	// AccMandated is a parameter the source code declares implicitly,
	// such as an inner class constructor's enclosing instance.
	AccMandated Access = 0x8000
)

// Has reports whether the set has every flag of another set. It
// allocates nothing.
func (a Access) Has(flags Access) bool { return a&flags == flags }
