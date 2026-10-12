// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads Java source into the node graph.
//
// [New] builds the frontend through the kit. The frontend claims every
// .java file. A unit is one directory, because a Java package is the files
// of one directory under a source root, so the main and the test directory
// of a package are two units that both contribute to the package. The
// shared input of each member is the nearest pom.xml above it. Every unit
// key folds the version of the frontend, the version of the grammar and
// the [Options].
//
// # Packages
//
// The package path of a file is its package clause with slashes for dots,
// as in com/acme/store. A file without a package clause is in the unnamed
// package, which has the empty path. Every package of a unit has the
// module of the governing pom.xml: gen.module with the group and the
// artifact, or with the group of the parent where the module states none,
// and gen.moduleRoot with the directory of the pom.xml. A pom.xml that does
// not read or parse reports under [BadPOM]. The Javadoc of a
// package-info.java file documents its package, and the annotations of its
// package clause annotate its File node. A module-info.java file declares
// no type, and java.module stamps its File node with the name of the
// module.
//
// # Declarations
//
// A class is a Struct, abstract, final and sealed where its modifiers state
// them, with its superclass, its interfaces and the subclasses that it
// permits. A record is a final Struct with the stamp java.record, and each
// component is a public field that is immutable. An interface is an
// Interface. An annotation type is an Interface with the stamp
// java.annotationType, and its elements are its methods. An enum is an
// Enum whose variants are its constants, and the value of each variant is
// its constructor arguments verbatim. A nested type is in the Types of its
// host, at the type level where it is static, in an interface or a record.
// A type that an enum declares reports under [UnmodeledItem], because an
// enum has no list of nested types.
//
// A field declaration declares one field per declarator, a static field at
// the type level and a final field immutable. A method is a Method, at the
// type level where it is static, with its type parameters, its parameters,
// its result and the exceptions that it throws. A void method has no
// result. An explicit receiver parameter is the receiver of the method and
// not a parameter. A constructor is a method that constructs, named after
// its type, and the compact constructor of a record takes the components
// of the record as its parameters. The members of an interface are
// public. Its fields are at the type level and immutable, its methods
// without a body are abstract, and a method marked default has a default.
// An annotation is an annotation of its declaration, with its name as
// written and its arguments verbatim. An initializer block, a local class
// and an anonymous class are part of a body, which the model does not
// contain.
//
// A compact source file, which has a method outside a type, implicitly
// declares a final class with package access in the unnamed package. The
// Java Language Specification leaves the name of the class to the host
// system, and the frontend names it after its file without the .java
// extension, as javac does. The methods, the fields and the types of the
// file are the members of the class, and a member class without static is
// an inner class.
//
// # Types
//
// The spelling of a reference is its tokens without the whitespace between
// them and without its type annotations. One space remains between two
// identifier tokens. T[] is a List, one per dimension, and the dimensions
// of a declarator and of a method are Lists too. A bounded wildcard is a
// Wildcard of its bound with Variance Out for extends and In for super,
// and an unbounded ? is a Wildcard without a child. A generic type keeps
// its bare name in the spelling and its arguments in Args. Every other
// type is Named with its spelling. A name that a single-type import binds
// records the package of the import as its package. The frontend declares
// that Java overloads, so the discriminator of a callable contains the
// types of its parameters, a variadic one followed by ....
//
// # Resolution
//
// Resolve returns tiers in the shadowing order of the Java Language
// Specification: the member types of each enclosing type, innermost first,
// the single-type imports, the types of the package of the file with the
// own types of the file, and the on-demand imports with java.lang. A
// static import resolves a type name as a type import does. A qualified
// name refers to a member type of its first name where that name is a type
// in scope. Otherwise it refers to a type of the package that its other
// names spell, and then to a type nested in a type that they spell.
//
// # Comments and carriers
//
// The documentation and the carriers of a declaration are in the Javadoc
// or the line comments directly above it, and in the comments among its
// modifiers. A run of line comments reads as one text, so the
// continuation of a carrier folds across the run. The comment on the last
// line of a declaration is its trailing comment, and its carriers attach
// too. A carrier that no declaration takes, or that is on a subject that
// the model cannot address, reports under [UnaddressedCarrier], and the
// finding describes the subject. A carrier that the kernel grammar refuses
// reports under [BadCarrier]. The comments in the body of a method or a
// constructor belong to its statements.
//
// # Markers
//
// An annotation whose name starts with the brand is a marker of a
// directive, as @acme.stub(tag = "test") and @acme.gen.table are. The
// frontend lifts its element values from the syntax: an element-value pair
// as a keyed argument, and the element of the single-element form as a
// positional one. A value is a string, a number with at most one minus
// sign, true, false or an array of them. A number lifts as the decimal
// text of its value, so the int 0xFFFFFFFF lifts as -1. The marker
// attaches its directive to the type, field, method, constructor, enum
// constant, annotation type element or package that it annotates, and it
// remains an annotation. A marker whose name is not a directive name, or
// whose value does not lift, reports under [BadMarker]. A text block does
// not lift. A marker on a parameter, or on a type that an enum declares,
// reports under [UnaddressedCarrier].
//
// # Classification
//
// java.testFile stamps a file under a src/test/ directory, the layout that
// Maven and Gradle share, and a file that the default includes of
// Surefire match: Test*.java, *Test.java, *Tests.java and *TestCase.java.
//
// java.generated stamps a file with a top-level type that has the
// annotation [java.GeneratedAnnotation] or
// [java.LegacyGeneratedAnnotation]. The annotation has its qualified name,
// or a simple name that a single-type import or an on-demand import of its
// package binds. The value of the stamp is the first such annotation as
// written. The frontend stamps a file once, also where the signature depth
// leaves the type out.
//
// # Syntax errors
//
// A syntax error is a problem of the source. The frontend reports the
// first [plugin.MaxSyntaxFindings] errors of a file under [UnparsedFile],
// each at its position, and one finding that counts the rest. Every
// declaration that the parser recovered loads.
//
// # Signature depth
//
// A load at signature depth keeps the public and the protected
// declarations and members, and leaves out the package-private and the
// private ones, each with its comments. The class of a compact source file
// has package access, so a load at signature depth leaves the members of
// the file out.
//
// # Dependencies
//
// The frontend has the dependent role over three stores, which [Stores]
// builds from the environment: the ct.sym of the JDK under [JDKStore], the
// Maven local repository under [MavenStore], and the module cache of Gradle
// under [GradleStore]. The first round returns one unit for each library
// that the Classpath of the [Options] lists as group:artifact:version. The
// unit is the JAR from the Maven store, whose SHA-1 record is the shared
// input of the unit, or the JAR from the Gradle store, whose directory has
// the digest of the JAR as its name. Either digest must match the JAR.
// Every round returns the JDK packages of its needs from the entries of
// ct.sym for the release of the options, or for the newest release that
// ct.sym lists where the options name none. The first round also returns
// java.lang. A need that is a class, as the need of a static import is,
// places the package of the class. A library that neither store has, a
// digest that does not match the JAR, and a release that ct.sym does not
// list fail the load.
//
// A need whose package neither ct.sym nor a classpath JAR has yields no
// unit, and the round reports it with the reason. The first round reads
// the packages of the JARs that it returns. The needs of a later round are
// packages that no loaded JAR declares.
//
// # Class files
//
// The members of a dependency unit are class files, ct.sym entries and
// JARs, which [classfile] decodes. A JAR provides each class from its
// highest META-INF/versions/N entry with N at most the release, where its
// manifest marks it multi-release. A class lowers to the declaration of
// its source: a class, a record, an interface, an annotation interface or
// an enum, with its supertypes, its type parameters, its fields and its
// methods. Object is left out as a superclass. A member class lowers into
// the class that the InnerClasses attribute names as its declaring class,
// and the member classes of an enum are left out. The load leaves out a
// local or an anonymous class, a synthetic or a bridge member, a class
// initializer and a private member, and at signature depth a member with
// package access. A constructor is named after its class. The name of a
// parameter is its name in MethodParameters, and empty without that
// attribute. The enclosing instance of an inner class constructor is not a
// parameter. An annotation has its binary name in dotted form, with one
// argument per element-value pair, as in value = "x". The spelling of a
// reference is the binary name of its class in dotted form, as in
// java.util.Map$Entry. The scope of its File node maps the spelling to the
// one identity of the class. Each File node imports the packages that its
// declarations reference, so the needs of the next round include them. A
// class file, a JAR or an entry of a JAR that does not read or decode
// reports under [BadClassFile], and the load continues without it. A JAR
// whose manifest does not read reports the same way and loads its root
// entries.
//
// # Dependency position
//
// lang/java/frontend imports the sdk facade, the satellite root,
// lang/treesitter and its Java grammar, lang/numeric, its classfile
// package, and the Go standard library. It runs no tool. [Stores] reads
// ct.sym when a composition calls it, and a load reads nothing outside the
// doors of its units and the readers of its rounds. The conformance corpus
// and a composition import it.
package frontend
