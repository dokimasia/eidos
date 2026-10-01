// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// storeSep separates a store's name from the path inside the store in
// a qualified path.
const storeSep = "://"

// ErrStoreAbsent reports a qualified path naming a store the load does
// not provide. A dependent frontend reads it as that source being off:
// the composition configured no such tree, which differs from a
// configured store that lacks a file.
var ErrStoreAbsent = errors.New("plugin: no such store")

// StorePath returns the qualified path of a file inside a store: the
// store's name, "://", and the slash path inside the store, as in
// "gomod://golang.org/x/mod@v0.41.0/modfile/rule.go". No workspace path
// is a qualified path, because [fs.ValidPath] refuses the empty element
// that "//" spells.
func StorePath(store, path string) string { return store + storeSep + path }

// CutStorePath splits a qualified path into its store and the path
// inside the store, and reports false for a workspace path. The root
// of a store is the qualified path with nothing after the separator.
func CutStorePath(qualified string) (store, path string, ok bool) {
	store, path, ok = strings.Cut(qualified, storeSep)
	if !ok || !ValidStoreName(store) {
		return "", "", false
	}
	return store, path, true
}

// ValidStoreName reports whether a name can name a store: it is not
// empty, and it contains neither the colon nor the slash that a
// qualified path separates on.
func ValidStoreName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ":/")
}

// StoreFS is a workspace tree with named stores beside it. The load
// hands every unit a StoreFS, and a unit resolves a qualified path
// through it.
type StoreFS interface {
	fs.FS

	// Store returns one named store, and false for a name the load
	// does not provide.
	Store(name string) (fs.FS, bool)
}

// ReadFile returns the bytes of one file: a workspace path from the
// tree itself, and a qualified path from the store it names, which the
// tree provides as a [StoreFS]. A qualified path whose store the tree
// does not provide returns an error wrapping [ErrStoreAbsent].
func ReadFile(fsys fs.FS, path string) ([]byte, error) {
	tree, inner, err := treeOf(fsys, path)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(tree, inner)
}

// ReadDir returns one directory's entries sorted by name, and resolves
// a qualified path the way [ReadFile] does. The root of a store is its
// qualified path with nothing after the separator.
func ReadDir(fsys fs.FS, path string) ([]fs.DirEntry, error) {
	tree, inner, err := treeOf(fsys, path)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(tree, inner)
}

// treeOf returns the tree a path resolves in and the path inside that
// tree.
func treeOf(fsys fs.FS, path string) (fs.FS, string, error) {
	store, inner, qualified := CutStorePath(path)
	if !qualified {
		return fsys, path, nil
	}
	if inner == "" {
		inner = "."
	}
	if stores, provides := fsys.(StoreFS); provides {
		if tree, held := stores.Store(store); held {
			return tree, inner, nil
		}
	}
	return nil, "", fmt.Errorf("%w: %s names store %q", ErrStoreAbsent, path, store)
}
