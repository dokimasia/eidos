// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads Java source into the node graph.
//
// [New] builds the frontend through the kit. The claim is every .java
// file. A unit is one directory, because a Java package is the files of
// one directory under a source root, and a package's main and test
// directories are two units that both contribute to the package. Each
// member's shared input is the nearest pom.xml above it. Every unit key
// folds the frontend's version, the grammar's and the [Options].
//
// # Packages
//
// A file's package path is its package clause with slashes for dots, as
// in com/acme/store, and a file without one is in the unnamed package,
// with the empty path. The module the governing pom.xml states stamps
// every package a unit declares into: gen.module with the group and the
// artifact, the parent's group where the module states none, and
// gen.moduleRoot with the pom.xml's directory. A pom.xml that does not
// read or parse reports under [BadPOM]. The Javadoc of a
// package-info.java file documents its package, and the annotations of
// its package clause annotate its File node. A module-info.java file
// declares no type, and java.module stamps its File node with the
// module's name.
//
// # Declarations
//
// A class is a Struct, abstract, final and sealed where its modifiers
// state them, with its superclass, its interfaces and the subclasses it
// permits. A record is a Struct, final and stamped java.record, each
// component a public field that is immutable. An interface is an
// Interface, an annotation type an Interface stamped java.annotationType
// whose elements are its methods, and an enum an Enum whose constants
// are its variants, each one's value its constructor arguments
// verbatim. A nested type is in its host's Types, at the type level
// where it is static, in an interface, or a record, and a type an enum
// declares reports under [UnmodeledItem], because an enum has no list of
// nested types.
//
// A field declaration declares one field per declarator, a static field
// at the type level and a final one immutable. A method is a Method,
// static at the type level, with its type parameters, its parameters,
// its result and the exceptions it throws, and a void method has no
// result. An explicit receiver parameter is the method's receiver and no
// parameter. A constructor is a method that constructs, named after its
// type, and a record's compact constructor takes the record's components
// as its parameters. An interface's members are public, its fields at
// the type level and immutable, its method without a body abstract and
// one marked default with a default. An annotation is an annotation of
// its declaration, its name as written and its arguments verbatim. An
// initializer block, a local class and an anonymous class are part of a
// body, which the model does not contain.
//
// A compact source file, one with a method outside a type, implicitly
// declares a final class with package access in the unnamed package.
// The Java Language Specification leaves the class's name to the host
// system, and the frontend names it after its file without the .java
// extension, as javac does. The file's methods, fields and types are
// the class's members, and a member class without static is an inner
// class.
//
// # Types
//
// A reference spells its tokens without the whitespace between them,
// one space kept between two identifier tokens, and without its type
// annotations. T[] is a List, one per dimension, as a declarator's and a
// method's dimensions state them too, a bounded wildcard a Wildcard of
// its bound with Variance Out for extends and In for super, and an
// unbounded ? a Wildcard without a child. A generic type keeps its
// bare name in the spelling and its arguments in Args. Every other type
// is Named with its spelling, and a name a single-type import binds
// records the package it imports from as its package. The frontend
// declares that Java overloads, and a callable's discriminator spells
// its parameters' types, a variadic one behind ....
//
// # Resolution
//
// Resolve returns tiers in the shadowing order of the Java Language
// Specification: the member types of each enclosing type, innermost
// first, the single-type imports, the types of the file's package, the
// file's own among them, and the on-demand imports with java.lang. A
// static import resolves a type name the way a type import does. A
// qualified name names a member type of its first name where that name
// is a type in scope, and then a type of the package its other names
// spell before a type nested in a type they spell.
//
// # Comments and carriers
//
// A declaration's documentation and carriers are the Javadoc or line
// comments directly above it, and the comments among its modifiers. A
// run of line comments reads as one text, so a carrier's continuation
// folds across them. The comment on a declaration's last line is its
// trailing comment, and its carriers attach too. A carrier no
// declaration takes reports under [UnaddressedCarrier], and one the
// kernel grammar refuses under [BadCarrier]. The comments in a method's
// or a constructor's body belong to its statements.
//
// # Markers
//
// An annotation whose name starts with the brand is a marker of a
// directive, as @acme.stub(tag = "test") and @acme.gen.table are. The
// frontend lifts its element values from the syntax: an element-value
// pair as a keyed argument, and the single element of the single-element
// form as a positional one. A value is a string, a number with at most
// one minus sign, true, false or an array of them. A number lifts as the
// decimal text of its value, so the int 0xFFFFFFFF lifts as -1. The
// marker attaches its directive to the type, field, method, constructor,
// enum constant, annotation type element or package that it annotates,
// and it remains an annotation. A marker whose name is not a directive
// name, or whose value does not lift, reports under [BadMarker]. A text
// block does not lift. A marker on a parameter, or on a type that an
// enum declares, reports under [UnaddressedCarrier].
//
// # Classification
//
// java.testFile stamps a file under a src/test/ directory, the layout
// Maven and Gradle share, and a file Surefire's default includes name:
// Test*.java, *Test.java, *Tests.java and *TestCase.java.
//
// # Signature depth
//
// A load at signature depth keeps the public and protected declarations
// and members, and leaves out the package-private and private ones, each
// with its comments. A compact source file's class has package access,
// so a load at signature depth leaves the file's members out.
//
// # Dependencies
//
// The frontend is in the dependent role over three stores, which
// [Stores] builds from the environment: the JDK's ct.sym under
// [JDKStore], the Maven local repository under [MavenStore], and
// Gradle's module cache under [GradleStore]. The first round returns one
// unit per library [Options] Classpath names, group:artifact:version
// each: its JAR from the Maven store, whose SHA-1 record is the unit's
// shared input, or from the Gradle store, whose directory names the JAR's
// digest. Either digest must be the JAR's. Every round returns the JDK
// packages its needs name, each from ct.sym's entries for the release
// the options name, the newest ct.sym lists where they name none, and
// the first round returns java.lang whatever the needs are. A need that
// names a class, as a static import's does, places the class's package.
// A library neither store has, a digest that is not the JAR's, and a
// release ct.sym does not list fail the load.
//
// A need whose package neither ct.sym nor a classpath JAR has yields no
// unit, and the round reports it with the reason. The first round reads
// the packages of the JARs it returns, and a later round's needs are
// packages no loaded JAR declares.
//
// # Class files
//
// A dependency unit's members are class files, ct.sym entries and JARs,
// which [classfile] decodes, and a JAR provides each class from its
// highest META-INF/versions/N entry with N at most the release where its
// manifest marks it multi-release. A class lowers to what its source
// declares: a class, a record, an interface, an annotation interface or
// an enum, with its supertypes, its type parameters, its fields and its
// methods, Object left out as a superclass. A member class lowers into
// the class the InnerClasses attribute names as declaring it, and an
// enum's member classes are left out. A local or anonymous class, a
// synthetic or bridge member, a class initializer and a private member
// are left out, and at signature depth so is a member with package
// access. A constructor is named after its class. A parameter's name is
// its MethodParameters name, empty without one, and an inner class
// constructor's enclosing instance is no parameter. An annotation is
// named by its binary name in dotted form, with one argument per
// element-value pair, as in value = "x". A reference spells its class's
// binary name in dotted form, as in java.util.Map$Entry, and its File
// node's scope maps the spelling to the one identity the class has. Each
// File node imports the packages its declarations reference, so the next
// round's needs include them. A class file, a JAR or a JAR's entry that
// does not read or decode reports under [BadClassFile], and the load
// continues without it. A JAR whose manifest does not read reports the
// same way, and loads its root entries.
//
// # Dependency position
//
// lang/java/frontend imports the sdk facade, the satellite root,
// lang/treesitter and its Java grammar, lang/numeric, its classfile
// package, and the Go stdlib. It runs no tool. [Stores] reads ct.sym when a composition
// calls it, and a load reads nothing outside its units' doors and its
// rounds' readers. The conformance corpus and a composition import it.
package frontend
