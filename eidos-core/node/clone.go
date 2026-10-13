// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node

// CloneRef returns a deep copy of r, and nil for a nil reference. The copy
// has the facts of r and a copy of each node below r: the children, the
// type arguments and the absent members, and the fields and the methods of
// an inline body with their receivers, parameters, results, type
// parameters and thrown types. The copy shares no node with r, so a
// frontend that lists one type in two places of a graph lists two nodes,
// as the graph requires. The lists of text and of annotations of a member
// are shared, because no step writes them after the parse. A nil member of
// a list stays nil in the copy.
//
// # Allocation contract
//
// CloneRef allocates each node that it copies and each list of nodes, and
// nothing for a nil reference.
func CloneRef(r *TypeRef) *TypeRef {
	if r == nil {
		return nil
	}
	c := *r
	c.Elems, c.Args, c.Absent = cloneRefs(r.Elems), cloneRefs(r.Args), cloneRefs(r.Absent)
	if r.Fields != nil {
		c.Fields = make([]*Field, len(r.Fields))
		for i, f := range r.Fields {
			if f != nil {
				copied := *f
				copied.Type = CloneRef(f.Type)
				c.Fields[i] = &copied
			}
		}
	}
	if r.Methods != nil {
		c.Methods = make([]*Method, len(r.Methods))
		for i, m := range r.Methods {
			if m != nil {
				c.Methods[i] = cloneMethod(m)
			}
		}
	}
	return &c
}

// cloneMethod returns a copy of the method m of an inline body, with a
// copy of each node below it, as [CloneRef] copies them.
func cloneMethod(m *Method) *Method {
	c := *m
	c.Receiver = cloneParam(m.Receiver)
	c.Receives = CloneRef(m.Receives)
	c.Throws = cloneRefs(m.Throws)
	if m.TypeParams != nil {
		c.TypeParams = make([]*TypeParam, len(m.TypeParams))
		for i, tp := range m.TypeParams {
			if tp != nil {
				copied := *tp
				copied.Bounds = cloneRefs(tp.Bounds)
				copied.Default = CloneRef(tp.Default)
				copied.Type = CloneRef(tp.Type)
				c.TypeParams[i] = &copied
			}
		}
	}
	if m.Params != nil {
		c.Params = make([]*Param, len(m.Params))
		for i, p := range m.Params {
			c.Params[i] = cloneParam(p)
		}
	}
	if m.Returns != nil {
		c.Returns = make([]*Return, len(m.Returns))
		for i, ret := range m.Returns {
			if ret != nil {
				copied := *ret
				copied.Type = CloneRef(ret.Type)
				c.Returns[i] = &copied
			}
		}
	}
	return &c
}

// cloneParam returns a copy of the parameter p with a copy of its type, and
// nil for a nil parameter.
func cloneParam(p *Param) *Param {
	if p == nil {
		return nil
	}
	c := *p
	c.Type = CloneRef(p.Type)
	return &c
}

// cloneRefs returns a list of a copy of each reference of refs, in order,
// and nil for a nil list.
func cloneRefs(refs []*TypeRef) []*TypeRef {
	if refs == nil {
		return nil
	}
	out := make([]*TypeRef, len(refs))
	for i, r := range refs {
		out[i] = CloneRef(r)
	}
	return out
}
