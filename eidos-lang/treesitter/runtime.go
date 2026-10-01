// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter

/*
#cgo noescape ts_tree_cursor_delete
#cgo noescape ts_tree_cursor_goto_first_child
#cgo noescape ts_tree_cursor_goto_next_sibling
#cgo noescape ts_tree_cursor_goto_parent
#cgo noescape ts_tree_cursor_current_node
#cgo noescape eidos_step
#cgo noescape eidos_field_step
#cgo noescape eidos_tokens
#cgo nocallback ts_node_symbol
#cgo nocallback ts_node_is_named
#cgo nocallback ts_node_is_missing
#cgo nocallback ts_node_is_extra
#cgo nocallback ts_node_is_error
#cgo nocallback ts_node_has_error
#cgo nocallback ts_node_start_byte
#cgo nocallback ts_node_end_byte
#cgo nocallback ts_node_start_point
#cgo nocallback ts_node_end_point
#cgo nocallback ts_node_child_by_field_id
#cgo nocallback ts_node_parent
#cgo nocallback ts_node_next_named_sibling
#cgo nocallback ts_node_prev_named_sibling
#cgo nocallback ts_tree_cursor_new
#cgo nocallback ts_tree_cursor_delete
#cgo nocallback ts_tree_cursor_goto_first_child
#cgo nocallback ts_tree_cursor_goto_next_sibling
#cgo nocallback ts_tree_cursor_goto_parent
#cgo nocallback ts_tree_cursor_current_node
#cgo nocallback eidos_step
#cgo nocallback eidos_field_step
#cgo nocallback eidos_tokens

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

typedef struct TSLanguage TSLanguage;
typedef struct TSTree TSTree;

// The runtime's node, tree cursor and point, laid out as the runtime's
// tree_sitter/api.h declares them. The layer keeps each in its own
// memory and passes it by value.
typedef struct TSNode {
	uint32_t context[4];
	const void *id;
	const TSTree *tree;
} TSNode;

typedef struct TSTreeCursor {
	const void *tree;
	const void *id;
	uint32_t context[3];
} TSTreeCursor;

typedef struct TSPoint {
	uint32_t row;
	uint32_t column;
} TSPoint;

// EidosSpan is one token's first byte and the byte after its last.
typedef struct EidosSpan {
	uint32_t from;
	uint32_t to;
} EidosSpan;

// The runtime's functions the layer calls itself, as tree_sitter/api.h
// declares them, which the runtime binding compiles into every binary
// that links this package.
extern void ts_set_allocator(void *(*new_malloc)(size_t), void *(*new_calloc)(size_t, size_t),
	void *(*new_realloc)(void *, size_t), void (*new_free)(void *));
extern uint16_t ts_language_symbol_for_name(const TSLanguage *self, const char *name, uint32_t length, bool named);
extern uint16_t ts_node_symbol(TSNode self);
extern bool ts_node_is_named(TSNode self);
extern bool ts_node_is_missing(TSNode self);
extern bool ts_node_is_extra(TSNode self);
extern bool ts_node_is_error(TSNode self);
extern bool ts_node_has_error(TSNode self);
extern uint32_t ts_node_start_byte(TSNode self);
extern uint32_t ts_node_end_byte(TSNode self);
extern TSPoint ts_node_start_point(TSNode self);
extern TSPoint ts_node_end_point(TSNode self);
extern uint32_t ts_node_child_count(TSNode self);
extern TSNode ts_node_child_by_field_id(TSNode self, uint16_t field_id);
extern TSNode ts_node_parent(TSNode self);
extern TSNode ts_node_next_named_sibling(TSNode self);
extern TSNode ts_node_prev_named_sibling(TSNode self);
extern TSTreeCursor ts_tree_cursor_new(TSNode node);
extern void ts_tree_cursor_delete(TSTreeCursor *self);
extern bool ts_tree_cursor_goto_first_child(TSTreeCursor *self);
extern bool ts_tree_cursor_goto_next_sibling(TSTreeCursor *self);
extern bool ts_tree_cursor_goto_parent(TSTreeCursor *self);
extern TSNode ts_tree_cursor_current_node(const TSTreeCursor *self);
extern uint16_t ts_tree_cursor_current_field_id(const TSTreeCursor *self);

// eidos_none returns the null node, whose id is NULL.
static TSNode eidos_none(void) {
	TSNode none = {{0, 0, 0, 0}, NULL, NULL};
	return none;
}

// eidos_step moves a cursor to its node's first child where first is
// set and to the next sibling otherwise, past every child that is not
// named where named is set, and returns the node it stops on, the null
// node past the last child.
static TSNode eidos_step(TSTreeCursor *c, bool first, bool named) {
	bool more = first ? ts_tree_cursor_goto_first_child(c) : ts_tree_cursor_goto_next_sibling(c);
	for (; more; more = ts_tree_cursor_goto_next_sibling(c)) {
		TSNode n = ts_tree_cursor_current_node(c);
		if (!named || ts_node_is_named(n)) {
			return n;
		}
	}
	return eidos_none();
}

// eidos_field_step is eidos_step over the children under one field.
static TSNode eidos_field_step(TSTreeCursor *c, bool first, uint16_t field) {
	bool more = first ? ts_tree_cursor_goto_first_child(c) : ts_tree_cursor_goto_next_sibling(c);
	for (; more; more = ts_tree_cursor_goto_next_sibling(c)) {
		if (ts_tree_cursor_current_field_id(c) == field) {
			return ts_tree_cursor_current_node(c);
		}
	}
	return eidos_none();
}

// eidos_tokens writes the span of each token of a node's subtree that
// spans source, in source order and without the subtree's extras, into
// out, at most cap of them, and returns how many the subtree has. A
// token is a node without children. The cursor's root is the node, so
// the walk ends where it would leave the subtree.
static uint32_t eidos_tokens(TSNode root, EidosSpan *out, uint32_t cap) {
	TSTreeCursor c = ts_tree_cursor_new(root);
	uint32_t n = 0;
	for (;;) {
		TSNode node = ts_tree_cursor_current_node(&c);
		bool extra = ts_node_is_extra(node);
		if (!extra && ts_node_child_count(node) == 0) {
			uint32_t from = ts_node_start_byte(node);
			uint32_t to = ts_node_end_byte(node);
			if (from < to) {
				if (n < cap) {
					out[n].from = from;
					out[n].to = to;
				}
				n++;
			}
		} else if (!extra && ts_tree_cursor_goto_first_child(&c)) {
			continue;
		}
		while (!ts_tree_cursor_goto_next_sibling(&c)) {
			if (!ts_tree_cursor_goto_parent(&c)) {
				ts_tree_cursor_delete(&c);
				return n;
			}
		}
	}
}
*/
import "C"

import (
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// The binding's Node is the runtime's TSNode and nothing else, which
// rootOf reads as the layer's own: the package compiles only while the
// two sizes are equal.
const (
	_ = unsafe.Sizeof(ts.Node{}) - unsafe.Sizeof(C.TSNode{})
	_ = unsafe.Sizeof(C.TSNode{}) - unsafe.Sizeof(ts.Node{})
)

// rawNode is the runtime's syntax node as a value: four context words,
// the node's id, which is nil for the null node, and its tree.
type rawNode = C.TSNode

// rawPoint is the runtime's zero-based row and byte column.
type rawPoint = C.TSPoint

// span is one token's first byte and the byte after its last, as
// eidos_tokens writes it.
type span = C.EidosSpan

// cursor is the runtime's tree cursor over one node's subtree, in the
// caller's memory. Its owner deletes it once.
type cursor struct {
	c C.TSTreeCursor
}

// init sets the runtime's allocator back to its default, libc's. The
// binding routes every allocation of the runtime through a function of
// its own in Go, which adds a call back into Go to each one, and both
// allocate from libc, so memory one allocated the other frees. With no
// call back into Go left, the runtime functions the package declares are
// nocallback, and the ones that take a cursor or a buffer of the
// package's memory noescape, so the compiler keeps that memory on the
// caller's stack.
func init() {
	C.ts_set_allocator(nil, nil, nil, nil)
}

// publicKind returns the id the runtime reports for every node of a
// named kind, and zero for a name that no named kind spells. The
// runtime reads the name's bytes during the call and keeps nothing.
func publicKind(lang *ts.Language, name string) Kind {
	return Kind(C.ts_language_symbol_for_name(
		(*C.TSLanguage)(unsafe.Pointer(lang.Inner)),
		(*C.char)(unsafe.Pointer(unsafe.StringData(name))),
		C.uint32_t(len(name)),
		true,
	))
}

// rootOf returns the root node of a tree the binding parsed, read as the
// layer's node.
func rootOf(t *ts.Tree) rawNode {
	return *(*rawNode)(unsafe.Pointer(t.RootNode()))
}

// present reports whether a node is not the runtime's null node.
func present(n rawNode) bool { return n.id != nil }

// symbolOf returns the kind of a node.
func symbolOf(n rawNode) Kind { return Kind(C.ts_node_symbol(n)) }

// isNamed reports whether a node is of a named kind.
func isNamed(n rawNode) bool { return bool(C.ts_node_is_named(n)) }

// isError reports whether a node is an ERROR node.
func isError(n rawNode) bool { return bool(C.ts_node_is_error(n)) }

// isMissing reports whether the parser inserted a node to recover.
func isMissing(n rawNode) bool { return bool(C.ts_node_is_missing(n)) }

// isExtra reports whether a node is an extra of the grammar.
func isExtra(n rawNode) bool { return bool(C.ts_node_is_extra(n)) }

// hasError reports whether a node's subtree has an ERROR or MISSING
// node.
func hasError(n rawNode) bool { return bool(C.ts_node_has_error(n)) }

// startByte returns a node's first byte.
func startByte(n rawNode) uint32 { return uint32(C.ts_node_start_byte(n)) }

// endByte returns the byte after a node's last.
func endByte(n rawNode) uint32 { return uint32(C.ts_node_end_byte(n)) }

// startPoint returns where a node starts.
func startPoint(n rawNode) rawPoint { return C.ts_node_start_point(n) }

// endPoint returns where a node ends.
func endPoint(n rawNode) rawPoint { return C.ts_node_end_point(n) }

// childByField returns a node's first child under a field, and the null
// node where it has none.
func childByField(n rawNode, f Field) rawNode { return C.ts_node_child_by_field_id(n, C.uint16_t(f)) }

// parentOf returns a node's parent, and the null node for the root.
func parentOf(n rawNode) rawNode { return C.ts_node_parent(n) }

// nextNamed returns the named sibling after a node, and the null node
// where there is none.
func nextNamed(n rawNode) rawNode { return C.ts_node_next_named_sibling(n) }

// prevNamed returns the named sibling before a node, and the null node
// where there is none.
func prevNamed(n rawNode) rawNode { return C.ts_node_prev_named_sibling(n) }

// tokens writes the spans of the tokens of a node's subtree into out, as
// eidos_tokens does, and returns how many the subtree has, which can
// exceed len(out).
func tokens(n rawNode, out []span) int {
	var at *span
	if len(out) > 0 {
		at = &out[0]
	}
	return int(C.eidos_tokens(n, at, C.uint32_t(len(out))))
}

// newCursor returns a cursor whose root is a node.
func newCursor(n rawNode) cursor { return cursor{C.ts_tree_cursor_new(n)} }

// close deletes the cursor's memory in the runtime.
func (c *cursor) close() { C.ts_tree_cursor_delete(&c.c) }

// step moves the cursor as eidos_step does and returns the node it stops
// on.
func (c *cursor) step(first, named bool) rawNode {
	return C.eidos_step(&c.c, C.bool(first), C.bool(named))
}

// fieldStep moves the cursor as eidos_field_step does and returns the
// node it stops on.
func (c *cursor) fieldStep(first bool, f Field) rawNode {
	return C.eidos_field_step(&c.c, C.bool(first), C.uint16_t(f))
}

// node returns the node the cursor is on.
func (c *cursor) node() rawNode { return C.ts_tree_cursor_current_node(&c.c) }

// firstChild moves the cursor to its node's first child, and reports
// whether the node has one.
func (c *cursor) firstChild() bool { return bool(C.ts_tree_cursor_goto_first_child(&c.c)) }

// nextSibling moves the cursor to its node's next sibling, and reports
// whether the node has one inside the cursor's subtree.
func (c *cursor) nextSibling() bool { return bool(C.ts_tree_cursor_goto_next_sibling(&c.c)) }

// parent moves the cursor to its node's parent, and reports whether the
// node has one inside the cursor's subtree.
func (c *cursor) parent() bool { return bool(C.ts_tree_cursor_goto_parent(&c.c)) }
