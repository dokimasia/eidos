// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"strings"
)

// storeSep separates the name of a store from the path inside the store in
// a qualified path.
const storeSep = "://"

// The environment variables that contain the home directory of the user,
// as os.UserHomeDir reads them on each system.
const (
	envHome        = "HOME"
	envPlan9Home   = "home"
	envUserProfile = "USERPROFILE"
)

// The systems whose home directory is not in HOME.
const (
	goosWindows = "windows"
	goosPlan9   = "plan9"
)

// ErrStoreAbsent is the error for a qualified path whose store the load
// does not provide. A dependent frontend reads it as a source that is off.
// The composition configured no such tree, which differs from a configured
// store without the file.
var ErrStoreAbsent = errors.New("plugin: no such store")

// Absent reports whether err means that a read found nothing to read. That
// is the case when the file does not exist, or when the load does not
// provide the store of a qualified path. A frontend that probes for an
// optional file, such as a manifest of a dependency, reads either error as
// the absence of the file and every other error as a failed read. Absent
// allocates nothing.
func Absent(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrStoreAbsent)
}

// StorePath returns the qualified path of a file inside a store. The path
// is the name of the store, "://" and the slash path inside the store, as
// in "gomod://golang.org/x/mod@v0.41.0/modfile/rule.go". No workspace path
// is a qualified path, because [fs.ValidPath] refuses the empty element
// between the two slashes. StorePath allocates the joined path, one
// allocation.
func StorePath(store, path string) string { return store + storeSep + path }

// CutStorePath splits a qualified path into its store and the path inside
// the store, and reports false for a workspace path. The root of a store is
// the qualified path with nothing after the separator. It allocates
// nothing, because both parts share the bytes of the path.
func CutStorePath(qualified string) (store, path string, ok bool) {
	store, path, ok = strings.Cut(qualified, storeSep)
	if !ok || !ValidStoreName(store) {
		return "", "", false
	}
	return store, path, true
}

// ValidStoreName reports whether name is a valid store name. A valid name
// is not empty and contains neither a colon nor a slash, which separate the
// parts of a qualified path. It allocates nothing.
func ValidStoreName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ":/")
}

// StoreFS is a workspace tree with named stores beside it. The load passes
// a StoreFS to every unit, and a unit resolves a qualified path through it.
type StoreFS interface {
	fs.FS

	// Store returns one named store, and false for a name that the load
	// does not provide.
	Store(name string) (fs.FS, bool)
}

// StoreLocator is a frontend that locates the stores that its dependency
// units read, such as the module cache of Go. A command line passes the
// stores of every locator of a composition to a run.
type StoreLocator interface {
	// Stores returns the stores of the frontend under their names, rooted
	// where the toolchain of the language finds them. It reads the
	// environment through getenv.
	//
	// Error modes: Stores returns an error for a root that neither the
	// environment nor the rules of the toolchain locate.
	Stores(getenv func(key string) string) (map[string]fs.FS, error)
}

// UserHomeVar returns the name of the environment variable that contains
// the home directory of the user, by the rule of os.UserHomeDir. The
// variable is USERPROFILE on Windows, home on Plan 9 and HOME on every other
// system. A [StoreLocator] reads the home directory through its getenv
// under this name. Where the variable is unset, the error of the locator
// contains the name. UserHomeVar allocates nothing.
func UserHomeVar() string {
	switch runtime.GOOS {
	case goosWindows:
		return envUserProfile
	case goosPlan9:
		return envPlan9Home
	}
	return envHome
}

// ReadFile returns the bytes of one file. It reads a workspace path from
// the tree itself, and a qualified path from its store, which the tree
// provides as a [StoreFS]. For a qualified path whose store the tree does
// not provide, it returns an error that wraps [ErrStoreAbsent].
//
// # Allocation contract
//
// ReadFile allocates what [fs.ReadFile] allocates in the tree or the
// store, and nothing of its own where the store exists.
func ReadFile(fsys fs.FS, path string) ([]byte, error) {
	tree, inner, err := treeOf(fsys, path)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(tree, inner)
}

// ReadDir returns the entries of one directory sorted by name, and resolves
// a qualified path as [ReadFile] does. The root of a store is its qualified
// path with nothing after the separator. It allocates what [fs.ReadDir]
// allocates, and nothing of its own where the store exists.
func ReadDir(fsys fs.FS, path string) ([]fs.DirEntry, error) {
	tree, inner, err := treeOf(fsys, path)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(tree, inner)
}

// Stat returns the stat of one file, and resolves a qualified path as
// [ReadFile] does. It allocates what [fs.Stat] allocates, and nothing of
// its own where the store exists.
func Stat(fsys fs.FS, path string) (fs.FileInfo, error) {
	tree, inner, err := treeOf(fsys, path)
	if err != nil {
		return nil, err
	}
	return fs.Stat(tree, inner)
}

// treeOf returns the tree in which a path resolves and the path inside that
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
	return nil, "", fmt.Errorf("%w: %s is in the store %q", ErrStoreAbsent, path, store)
}
