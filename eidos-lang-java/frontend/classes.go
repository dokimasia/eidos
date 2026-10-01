// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"maps"
	"path"
	"slices"
	"strings"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The extensions of a dependency unit's members: a ct.sym entry, a class
// file and a JAR.
const (
	sigExt   = ".sig"
	classExt = ".class"
	jarExt   = ".jar"
)

// The names the class-file lowering treats apart: a package's
// declaration, the methods that construct and initialize, and the
// supertypes source leaves unwritten, which Extends leaves out.
const (
	packageInfoClass = "package-info"
	initMethod       = "<init>"
	clinitMethod     = "<clinit>"
	objectClass      = "java/lang/Object"
	annotationClass  = "java/lang/annotation/Annotation"
)

// The spellings the class-file lowering writes: an unbounded wildcard
// and the openings of a bounded one, as the source lowering's compact
// form spells them, the separator of an annotation's element and value,
// and the package separator of a binary name in internal form.
const (
	wildcardMark  = "?"
	extendsBound  = "?extends "
	superBound    = "?super "
	elementEquals = " = "
	binarySlash   = "/"
)

// classFile is one decoded class of a dependency unit: the path of the
// member it is from, which its File node and positions name, and the
// class.
type classFile struct {
	path  string
	class *classfile.Class
}

// fileKey is one File node of a class-file unit: a member path in one
// package. A JAR's classes of one package share the JAR's File node in
// that package.
type fileKey struct {
	pkg  string
	path string
}

// classScope is a class file's bindings: the spelling of each class its
// references name, its binary name in dotted form, mapped to the one
// identity the class has. A class file names every class by its binary
// name, so a spelling has no other candidate.
type classScope map[string]symbol.Identity

// classHeader is what every class lowers the same way: its simple name,
// its visibility, whether it is a member class, whether a member class
// is static, and the class that declares a member class.
type classHeader struct {
	name   string
	vis    symbol.Visibility
	member bool
	static bool
	outer  string
}

// classLowering is one dependency unit's lowering of its class files:
// the unit, the classes by binary name, the member classes each class
// declares, in name order, and the File node of each member path in each
// package, in the order the lowering declares them, with each one's
// scope and the packages it imports.
type classLowering struct {
	u       *plugin.SourceUnit
	classes map[string]classFile
	members map[string][]string
	files   map[fileKey]*node.File
	order   []*node.File
	scopes  map[*node.File]classScope
	imports map[*node.File]map[string]bool
}

// lowerClasses lowers a unit's class files into its builder. A
// module-info class declares nothing, and a package-info class annotates
// its File node. A member class lowers into the class its InnerClasses
// entry names as declaring it, and a local or an anonymous class, whose
// entry names none, declares nothing. Every other class lowers into its
// package. Each File node imports the packages its declarations
// reference, so the next round's needs include them.
func lowerClasses(u *plugin.SourceUnit, classes []classFile) {
	l := &classLowering{
		u: u, classes: map[string]classFile{}, members: map[string][]string{}, files: map[fileKey]*node.File{},
		scopes: map[*node.File]classScope{}, imports: map[*node.File]map[string]bool{},
	}
	for _, cf := range classes {
		l.classes[cf.class.Name] = cf
	}
	var top []classFile
	for _, cf := range classes {
		c := cf.class
		if c.Access.Has(classfile.AccModule) {
			continue
		}
		file := l.fileOf(cf)
		self, nested := selfEntry(c)
		switch {
		case path.Base(c.Name) == packageInfoClass:
			file.Annotations = append(file.Annotations, annotationsOf(c.Annotations)...)
		case nested && self.Outer != "":
			l.members[self.Outer] = append(l.members[self.Outer], c.Name)
		case !nested:
			top = append(top, cf)
		}
	}
	for _, names := range l.members {
		slices.Sort(names)
	}
	for _, cf := range top {
		if decl := l.declare(cf, accessVisibility(cf.class.Access), nil); decl != nil {
			file := l.fileOf(cf)
			file.Decls = append(file.Decls, decl)
		}
	}
	for _, file := range l.order {
		for _, pkg := range slices.Sorted(maps.Keys(l.imports[file])) {
			file.Imports = append(file.Imports, &node.Import{Path: pkg, Wildcard: true, Pos: file.Pos})
		}
	}
}

// fileOf returns the File node of a class's member path in the class's
// package. The first call declares it in the package, with its scope.
func (l *classLowering) fileOf(cf classFile) *node.File {
	pkg := packagePath(cf.class.Name)
	key := fileKey{pkg: pkg, path: cf.path}
	if f, met := l.files[key]; met {
		return f
	}
	gb := l.u.Graph()
	p := gb.Package(pkg)
	if pkg != "" {
		p.Name = path.Base(pkg)
	}
	f := &node.File{Path: cf.path, Pos: classPos(cf.path)}
	p.Files = append(p.Files, f)
	scope := classScope{}
	gb.Scope(f, scope)
	l.files[key], l.scopes[f], l.imports[f] = f, scope, map[string]bool{}
	l.order = append(l.order, f)
	return f
}

// kept reports whether a load at the unit's depth keeps a declaration of
// a visibility: no private one, every other one at full depth, and a
// public or a protected one at signature depth.
func (l *classLowering) kept(vis symbol.Visibility) bool {
	if vis == symbol.VisibilityPrivate {
		return false
	}
	return l.u.Depth() == plugin.DepthFull || vis == symbol.VisibilityPublic || vis == symbol.VisibilityProtected
}

// declare lowers one class and the member classes it declares, and
// returns nil for a class the depth leaves out. vis is the class's
// visibility, and self the InnerClasses entry of a member class, nil for
// a top-level class.
func (l *classLowering) declare(cf classFile, vis symbol.Visibility, self *classfile.InnerClass) symbol.Symbol {
	if !l.kept(vis) {
		return nil
	}
	c := cf.class
	d := &classDecl{l: l, cf: cf, file: l.fileOf(cf), pos: classPos(cf.path), pkg: packagePath(c.Name)}
	d.scope = l.scopes[d.file]
	d.nest = map[string]classfile.InnerClass{}
	for _, ic := range c.Inner {
		d.nest[ic.Inner] = ic
	}
	h := classHeader{name: path.Base(c.Name), vis: vis}
	if self != nil {
		h.name, h.member, h.static, h.outer = self.Name, true, self.Access.Has(classfile.AccStatic), self.Outer
	}
	switch {
	case c.Access.Has(classfile.AccInterface):
		return d.iface(h)
	case c.Access.Has(classfile.AccEnum):
		return d.enum(h)
	case c.Record:
		return d.record(h)
	default:
		return d.class(h)
	}
}

// classDecl is one class file's lowering: the unit's lowering, the
// class, the File node and the scope its references resolve through, the
// position its declarations take, its package, and its InnerClasses
// entries by nested class, which split a binary name into its owners and
// its name.
type classDecl struct {
	l     *classLowering
	cf    classFile
	file  *node.File
	scope classScope
	pos   position.Pos
	pkg   string
	nest  map[string]classfile.InnerClass
}

// class lowers a class as a Struct: abstract, final and sealed where its
// flags state them, its type parameters, its superclass as what it
// extends unless it is Object, its interfaces, the subclasses it
// permits, and its members. A static member class is at the type level.
func (d *classDecl) class(h classHeader) *node.Struct {
	c := d.cf.class
	st := &node.Struct{
		Name: h.name, Pos: d.pos, Visibility: h.vis, Abstract: c.Access.Has(classfile.AccAbstract),
		Final: c.Access.Has(classfile.AccFinal), Sealed: c.Permitted != nil, Permits: d.classRefs(c.Permitted),
		Annotations: annotationsOf(c.Annotations),
	}
	if h.static {
		st.Level = symbol.LevelType
	}
	params, super, interfaces := d.supertypes()
	st.TypeParams = d.typeParams(params)
	if super.Kind == classfile.KindClass && super.BinaryName() != objectClass {
		st.Extends = []*node.TypeRef{d.ref(super)}
	}
	st.Implements = d.refs(interfaces)
	st.Fields, st.Methods = d.fields(), d.methods(h, false)
	st.Types = d.nested()
	return st
}

// iface lowers an interface, or an annotation interface, as an
// Interface: sealed where it permits subtypes, its type parameters, the
// interfaces it extends without the Annotation every annotation
// interface extends, and its members. java.annotationType stamps an
// annotation interface.
func (d *classDecl) iface(h classHeader) *node.Interface {
	c := d.cf.class
	it := &node.Interface{
		Name: h.name, Pos: d.pos, Visibility: h.vis, Sealed: c.Permitted != nil,
		Permits: d.classRefs(c.Permitted), Annotations: annotationsOf(c.Annotations),
	}
	params, _, interfaces := d.supertypes()
	it.TypeParams = d.typeParams(params)
	for _, t := range interfaces {
		if t.BinaryName() != annotationClass {
			it.Extends = append(it.Extends, d.ref(t))
		}
	}
	if c.Access.Has(classfile.AccAnnotation) {
		d.l.u.Graph().Stamp(it, meta.RawStamp{Key: java.AnnotationTypeKey, Value: true, Pos: d.pos})
	}
	it.Fields, it.Methods = d.fields(), d.methods(h, true)
	it.Types = d.nested()
	return it
}

// enum lowers an enum as an Enum: each enum constant a variant, which a
// class file states without its constructor arguments, and its other
// fields and its methods. An enum has no list of nested types, so its
// member classes are left out.
func (d *classDecl) enum(h classHeader) *node.Enum {
	c := d.cf.class
	e := &node.Enum{Name: h.name, Pos: d.pos, Visibility: h.vis, Annotations: annotationsOf(c.Annotations)}
	for _, f := range c.Fields {
		if f.Access.Has(classfile.AccEnum) {
			e.Variants = append(e.Variants, &node.EnumVariant{
				Name: f.Name, Pos: d.pos, Annotations: annotationsOf(f.Annotations),
			})
		}
	}
	e.Fields, e.Methods = d.fields(), d.methods(h, false)
	return e
}

// record lowers a record as a Struct, final and stamped java.record:
// each component a public field that is immutable, its type parameters,
// its interfaces and its members.
func (d *classDecl) record(h classHeader) *node.Struct {
	c := d.cf.class
	st := &node.Struct{
		Name: h.name, Pos: d.pos, Visibility: h.vis, Final: true, Annotations: annotationsOf(c.Annotations),
	}
	if h.static {
		st.Level = symbol.LevelType
	}
	params, _, interfaces := d.supertypes()
	st.TypeParams, st.Implements = d.typeParams(params), d.refs(interfaces)
	for _, comp := range c.Components {
		t := comp.Type
		if comp.Signature != nil {
			t = *comp.Signature
		}
		st.Fields = append(st.Fields, &node.Field{
			Name: comp.Name, Pos: d.pos, Visibility: symbol.VisibilityPublic, Mutability: symbol.MutabilityImmutable,
			Type: d.ref(t), Annotations: annotationsOf(comp.Annotations),
		})
	}
	d.l.u.Graph().Stamp(st, meta.RawStamp{Key: java.RecordKey, Value: true, Pos: d.pos})
	st.Fields = append(st.Fields, d.fields()...)
	st.Methods, st.Types = d.methods(h, false), d.nested()
	return st
}

// supertypes returns a class's type parameters, its superclass and its
// interfaces: the class signature's where the class has one, and the
// class file's names otherwise. The superclass is the zero Type for a
// class that names none.
func (d *classDecl) supertypes() ([]classfile.TypeParam, classfile.Type, []classfile.Type) {
	c := d.cf.class
	if sig := c.Signature; sig != nil {
		return sig.TypeParams, sig.Super, sig.Interfaces
	}
	var super classfile.Type
	if c.Super != "" {
		super = classType(c.Super)
	}
	interfaces := make([]classfile.Type, 0, len(c.Interfaces))
	for _, name := range c.Interfaces {
		interfaces = append(interfaces, classType(name))
	}
	return nil, super, interfaces
}

// fields lowers a class's fields, without its enum constants: each with
// its type, its constant as its value, and its annotations, static at
// the type level and final immutable, as an interface's fields are
// flagged. A synthetic field is left out, and so is one the depth leaves
// out.
func (d *classDecl) fields() []*node.Field {
	var out []*node.Field
	for _, f := range d.cf.class.Fields {
		vis := accessVisibility(f.Access)
		if f.Access.Has(classfile.AccSynthetic) || f.Access.Has(classfile.AccEnum) || !d.l.kept(vis) {
			continue
		}
		t := f.Type
		if f.Signature != nil {
			t = *f.Signature
		}
		fd := &node.Field{
			Name: f.Name, Pos: d.pos, Visibility: vis, Value: f.Value, Type: d.ref(t),
			Level: symbol.LevelInstance, Mutability: symbol.MutabilityMutable,
			Annotations: annotationsOf(f.Annotations),
		}
		if f.Access.Has(classfile.AccStatic) {
			fd.Level = symbol.LevelType
		}
		if f.Access.Has(classfile.AccFinal) {
			fd.Mutability = symbol.MutabilityImmutable
		}
		out = append(out, fd)
	}
	return out
}

// methods lowers a class's methods and constructors, and leaves out its
// class initializer, a synthetic method, a bridge among them, and a
// method the depth leaves out. A constructor is a method that
// constructs, named after its class. A static method is at the type
// level. An interface's method that is neither abstract nor static has a
// default.
func (d *classDecl) methods(h classHeader, iface bool) []*node.Method {
	var out []*node.Method
	for _, m := range d.cf.class.Methods {
		vis := accessVisibility(m.Access)
		if m.Name == clinitMethod || m.Access.Has(classfile.AccSynthetic) || !d.l.kept(vis) {
			continue
		}
		md := &node.Method{
			Name: m.Name, Pos: d.pos, Visibility: vis, Final: m.Access.Has(classfile.AccFinal),
			Abstract: m.Access.Has(classfile.AccAbstract), Annotations: annotationsOf(m.Annotations),
		}
		constructs := m.Name == initMethod
		if constructs {
			md.Name, md.Constructs = h.name, true
		}
		if m.Access.Has(classfile.AccStatic) {
			md.Level = symbol.LevelType
		}
		md.HasDefault = iface && !md.Abstract && md.Level != symbol.LevelType
		result, throws := m.Result, classTypes(m.Exceptions)
		if sig := m.Signature; sig != nil {
			md.TypeParams, result = d.typeParams(sig.TypeParams), sig.Result
			if len(sig.Throws) > 0 {
				throws = sig.Throws
			}
		}
		md.Params = d.params(m, constructs && h.member && !h.static, h.outer)
		if result != nil {
			ref := d.ref(*result)
			md.Returns = []*node.Return{{Pos: ref.Pos, Type: ref}}
		}
		md.Throws = d.refs(throws)
		out = append(out, md)
	}
	return out
}

// params lowers a method's parameters as source declares them: the
// descriptor's without the leading ones source does not declare, each
// with the signature's type where the method has a signature, its name
// from MethodParameters, empty without one, and its annotations. A
// variadic method's last parameter is positionally variadic, and its
// type is the element its array collects. innerCtor reports a
// constructor of an inner class, whose enclosing instance, of the class
// outer, the descriptor states first.
func (d *classDecl) params(m classfile.Method, innerCtor bool, outer string) []*node.Param {
	implicit, types := sourceParams(m, innerCtor, outer)
	var out []*node.Param
	for i, t := range types {
		at := implicit + i
		p := &node.Param{Pos: d.pos, Type: d.ref(t)}
		if len(m.Parameters) == len(m.Params) {
			p.Name = m.Parameters[at].Name
		}
		if j := at - (len(m.Params) - len(m.ParamAnnotations)); j >= 0 {
			p.Annotations = annotationsOf(m.ParamAnnotations[j])
		}
		out = append(out, p)
	}
	if last := len(out) - 1; last >= 0 && m.Access.Has(classfile.AccVarargs) && out[last].Type.Form == symbol.FormList {
		out[last].Variadic, out[last].Type = symbol.VariadicPositional, out[last].Type.Elems[0]
	}
	return out
}

// typeParams lowers type parameters: each one's name and bounds, without
// the Object bound a class file states for a parameter source bounds by
// nothing.
func (d *classDecl) typeParams(params []classfile.TypeParam) []*node.TypeParam {
	var out []*node.TypeParam
	for _, p := range params {
		tp := &node.TypeParam{Name: p.Name, Pos: d.pos}
		bounds := p.Bounds
		if len(bounds) == 1 && bounds[0].BinaryName() == objectClass {
			bounds = nil
		}
		tp.Bounds = d.refs(bounds)
		out = append(out, tp)
	}
	return out
}

// nested lowers the member classes a class declares, in name order, each
// at the visibility its InnerClasses entry states.
func (d *classDecl) nested() node.Symbols {
	var out node.Symbols
	for _, name := range d.l.members[d.cf.class.Name] {
		m := d.l.classes[name]
		self, _ := selfEntry(m.class)
		if decl := d.l.declare(m, accessVisibility(self.Access), &self); decl != nil {
			out = append(out, decl)
		}
	}
	return out
}

// refs lowers types into references.
func (d *classDecl) refs(types []classfile.Type) []*node.TypeRef {
	var out []*node.TypeRef
	for _, t := range types {
		out = append(out, d.ref(t))
	}
	return out
}

// classRefs lowers binary names into references to their classes.
func (d *classDecl) classRefs(names []string) []*node.TypeRef {
	return d.refs(classTypes(names))
}

// ref lowers a class file's type into a reference: a base type and a
// type variable Named by their names, an array a List, and a class Named
// by its binary name in dotted form, with its package and the type
// arguments of its last name. The scope maps a class's spelling to its
// identity, and the File node imports its package.
func (d *classDecl) ref(t classfile.Type) *node.TypeRef {
	switch t.Kind {
	case classfile.KindBase:
		return &node.TypeRef{Spelling: t.Keyword(), Pos: d.pos}
	case classfile.KindVar:
		return &node.TypeRef{Spelling: t.Var, Pos: d.pos}
	case classfile.KindArray:
		return listOf(d.ref(*t.Elem))
	}
	binary := t.BinaryName()
	ref := &node.TypeRef{Spelling: dotted(binary), Pos: d.pos, Package: packagePath(binary)}
	d.scope[ref.Spelling] = d.identityOf(binary)
	if ref.Package != "" && ref.Package != d.pkg {
		d.l.imports[d.file][ref.Package] = true
	}
	for _, a := range t.Class[len(t.Class)-1].Args {
		ref.Args = append(ref.Args, d.arg(a))
	}
	return ref
}

// arg lowers a type argument: an unbounded wildcard a Wildcard without a
// child, a bounded one a Wildcard of its bound with Variance Out for
// extends and In for super, and any other argument its type.
func (d *classDecl) arg(a classfile.TypeArg) *node.TypeRef {
	if a.Bound == classfile.BoundAny {
		return &node.TypeRef{Spelling: wildcardMark, Pos: d.pos, Form: symbol.FormWildcard}
	}
	inner := d.ref(*a.Type)
	switch a.Bound {
	case classfile.BoundExtends:
		return &node.TypeRef{
			Spelling: extendsBound + spelled(inner), Pos: d.pos, Form: symbol.FormWildcard,
			Variance: symbol.VarianceOut, Elems: []*node.TypeRef{inner},
		}
	case classfile.BoundSuper:
		return &node.TypeRef{
			Spelling: superBound + spelled(inner), Pos: d.pos, Form: symbol.FormWildcard,
			Variance: symbol.VarianceIn, Elems: []*node.TypeRef{inner},
		}
	default:
		return inner
	}
}

// identityOf returns the identity of the class a binary name names: its
// package, and the owners and the name the class file's InnerClasses
// entries split the rest of the name into. A class no entry names as a
// member class is top-level, whatever its name contains. The walk up the
// declaring classes is bounded by the entries, so a cycle the entries
// state ends.
func (d *classDecl) identityOf(binary string) symbol.Identity {
	var names []string
	top := binary
	for range len(d.nest) {
		ic, nested := d.nest[top]
		if !nested || ic.Outer == "" {
			break
		}
		names = append(names, ic.Name)
		top = ic.Outer
	}
	id := symbol.Identity{Lang: Lang, Package: packagePath(top), Name: path.Base(top)}
	if len(names) == 0 {
		return id
	}
	slices.Reverse(names)
	id.Owner = strings.Join(append([]string{id.Name}, names[:len(names)-1]...), nameSeparator)
	id.Name = names[len(names)-1]
	return id
}

// decodeClass decodes one class file of a member, named in findings by
// its name in the member: a JAR's entry, or the member's own name. It
// reports a class file that does not decode under [BadClassFile] at the
// member, and nothing loads from it.
func decodeClass(u *plugin.SourceUnit, p, name string, data []byte) (classFile, bool) {
	c, err := classfile.Parse(data)
	if err != nil {
		u.Errorf(BadClassFile, classPos(p), "%s: %v", name, err)
		return classFile{}, false
	}
	return classFile{path: p, class: c}, true
}

// selfEntry returns the InnerClasses entry a class has for itself, which
// a nested class has, and reports whether it has one.
func selfEntry(c *classfile.Class) (classfile.InnerClass, bool) {
	for _, ic := range c.Inner {
		if ic.Inner == c.Name {
			return ic, true
		}
	}
	return classfile.InnerClass{}, false
}

// sourceParams returns how many of a method's leading descriptor
// parameters source does not declare, and the types of the ones it
// does. A signature states only the declared ones (§4.7.9.1), and a
// signature longer than the descriptor is ignored. Without one,
// MethodParameters flags the undeclared ones synthetic, and an inner
// class constructor's enclosing instance mandated. Without either, an
// inner class constructor's first parameter is its enclosing instance
// where its type is the class outer, as the Java Language Specification
// states (§8.8.1).
func sourceParams(m classfile.Method, innerCtor bool, outer string) (int, []classfile.Type) {
	desc := m.Params
	if sig := m.Signature; sig != nil && len(sig.Params) <= len(desc) {
		return len(desc) - len(sig.Params), sig.Params
	}
	n := 0
	switch {
	case len(m.Parameters) == len(desc):
		for n < len(desc) && implicitParam(m.Parameters[n], innerCtor) {
			n++
		}
	case innerCtor && len(desc) > 0 && desc[0].BinaryName() == outer:
		n = 1
	}
	return n, desc[n:]
}

// implicitParam reports whether MethodParameters marks a leading
// parameter as one source does not declare: a synthetic one, or an inner
// class constructor's mandated one, its enclosing instance.
func implicitParam(p classfile.Parameter, innerCtor bool) bool {
	return p.Access.Has(classfile.AccSynthetic) || innerCtor && p.Access.Has(classfile.AccMandated)
}

// accessVisibility returns the visibility access flags state: public,
// protected, private, and package access where they state none.
func accessVisibility(access classfile.Access) symbol.Visibility {
	switch {
	case access.Has(classfile.AccPublic):
		return symbol.VisibilityPublic
	case access.Has(classfile.AccProtected):
		return symbol.VisibilityProtected
	case access.Has(classfile.AccPrivate):
		return symbol.VisibilityPrivate
	default:
		return symbol.VisibilityPackage
	}
}

// annotationsOf lowers a class file's annotations: each named by its
// binary name in dotted form, with one argument per element-value pair,
// the element's name and its value as source spells it.
func annotationsOf(list []classfile.Annotation) symbol.Annotations {
	var out symbol.Annotations
	for _, a := range list {
		s := symbol.Annotation{Name: dotted(a.Type)}
		for _, e := range a.Elements {
			s.Args = append(s.Args, e.Name+elementEquals+e.Value)
		}
		out = append(out, s)
	}
	return out
}

// classType returns the class type of a binary name, without type
// arguments.
func classType(binary string) classfile.Type {
	return classfile.Type{Kind: classfile.KindClass, Class: []classfile.ClassName{{Name: binary}}}
}

// classTypes returns the class types of binary names.
func classTypes(names []string) []classfile.Type {
	out := make([]classfile.Type, 0, len(names))
	for _, name := range names {
		out = append(out, classType(name))
	}
	return out
}

// packagePath returns the package of a binary name in internal form,
// and empty for a class of the unnamed package.
func packagePath(binary string) string {
	pkg, _, found := strings.CutLast(binary, binarySlash)
	if !found {
		return ""
	}
	return pkg
}

// dotted returns a binary name in internal form with dots for its
// slashes, as in java.util.Map$Entry.
func dotted(binary string) string {
	return strings.ReplaceAll(binary, binarySlash, nameSeparator)
}
