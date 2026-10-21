// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"cmp"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// libraryFile is the name of one library of TypeScript, as the lib option
// and a reference directive write it, and the declaration file of the
// library.
type libraryFile struct {
	name string
	file string
}

// libraryFiles lists the names of the libraries of TypeScript 7.0.2 with the
// file of each, in the order of LibMap of typescript-go 7.0.2. typescript-go
// loads the files of a library in this order. The name of a fallback refers
// to the file of the library that replaced it.
var libraryFiles = []libraryFile{
	{name: "es5", file: "lib.es5.d.ts"},
	{name: "es6", file: "lib.es2015.d.ts"},
	{name: "es2015", file: "lib.es2015.d.ts"},
	{name: "es7", file: "lib.es2016.d.ts"},
	{name: "es2016", file: "lib.es2016.d.ts"},
	{name: "es2017", file: "lib.es2017.d.ts"},
	{name: "es2018", file: "lib.es2018.d.ts"},
	{name: "es2019", file: "lib.es2019.d.ts"},
	{name: "es2020", file: "lib.es2020.d.ts"},
	{name: "es2021", file: "lib.es2021.d.ts"},
	{name: "es2022", file: "lib.es2022.d.ts"},
	{name: "es2023", file: "lib.es2023.d.ts"},
	{name: "es2024", file: "lib.es2024.d.ts"},
	{name: "es2025", file: "lib.es2025.d.ts"},
	{name: "esnext", file: "lib.esnext.d.ts"},
	{name: "dom", file: "lib.dom.d.ts"},
	{name: "dom.iterable", file: "lib.dom.iterable.d.ts"},
	{name: "dom.asynciterable", file: "lib.dom.asynciterable.d.ts"},
	{name: "webworker", file: "lib.webworker.d.ts"},
	{name: "webworker.importscripts", file: "lib.webworker.importscripts.d.ts"},
	{name: "webworker.iterable", file: "lib.webworker.iterable.d.ts"},
	{name: "webworker.asynciterable", file: "lib.webworker.asynciterable.d.ts"},
	{name: "scripthost", file: "lib.scripthost.d.ts"},
	{name: "es2015.core", file: "lib.es2015.core.d.ts"},
	{name: "es2015.collection", file: "lib.es2015.collection.d.ts"},
	{name: "es2015.generator", file: "lib.es2015.generator.d.ts"},
	{name: "es2015.iterable", file: "lib.es2015.iterable.d.ts"},
	{name: "es2015.promise", file: "lib.es2015.promise.d.ts"},
	{name: "es2015.proxy", file: "lib.es2015.proxy.d.ts"},
	{name: "es2015.reflect", file: "lib.es2015.reflect.d.ts"},
	{name: "es2015.symbol", file: "lib.es2015.symbol.d.ts"},
	{name: "es2015.symbol.wellknown", file: "lib.es2015.symbol.wellknown.d.ts"},
	{name: "es2016.array.include", file: "lib.es2016.array.include.d.ts"},
	{name: "es2016.intl", file: "lib.es2016.intl.d.ts"},
	{name: "es2017.arraybuffer", file: "lib.es2017.arraybuffer.d.ts"},
	{name: "es2017.date", file: "lib.es2017.date.d.ts"},
	{name: "es2017.object", file: "lib.es2017.object.d.ts"},
	{name: "es2017.sharedmemory", file: "lib.es2017.sharedmemory.d.ts"},
	{name: "es2017.string", file: "lib.es2017.string.d.ts"},
	{name: "es2017.intl", file: "lib.es2017.intl.d.ts"},
	{name: "es2017.typedarrays", file: "lib.es2017.typedarrays.d.ts"},
	{name: "es2018.asyncgenerator", file: "lib.es2018.asyncgenerator.d.ts"},
	{name: "es2018.asynciterable", file: "lib.es2018.asynciterable.d.ts"},
	{name: "es2018.intl", file: "lib.es2018.intl.d.ts"},
	{name: "es2018.promise", file: "lib.es2018.promise.d.ts"},
	{name: "es2018.regexp", file: "lib.es2018.regexp.d.ts"},
	{name: "es2019.array", file: "lib.es2019.array.d.ts"},
	{name: "es2019.object", file: "lib.es2019.object.d.ts"},
	{name: "es2019.string", file: "lib.es2019.string.d.ts"},
	{name: "es2019.symbol", file: "lib.es2019.symbol.d.ts"},
	{name: "es2019.intl", file: "lib.es2019.intl.d.ts"},
	{name: "es2020.bigint", file: "lib.es2020.bigint.d.ts"},
	{name: "es2020.date", file: "lib.es2020.date.d.ts"},
	{name: "es2020.promise", file: "lib.es2020.promise.d.ts"},
	{name: "es2020.sharedmemory", file: "lib.es2020.sharedmemory.d.ts"},
	{name: "es2020.string", file: "lib.es2020.string.d.ts"},
	{name: "es2020.symbol.wellknown", file: "lib.es2020.symbol.wellknown.d.ts"},
	{name: "es2020.intl", file: "lib.es2020.intl.d.ts"},
	{name: "es2020.number", file: "lib.es2020.number.d.ts"},
	{name: "es2021.promise", file: "lib.es2021.promise.d.ts"},
	{name: "es2021.string", file: "lib.es2021.string.d.ts"},
	{name: "es2021.weakref", file: "lib.es2021.weakref.d.ts"},
	{name: "es2021.intl", file: "lib.es2021.intl.d.ts"},
	{name: "es2022.array", file: "lib.es2022.array.d.ts"},
	{name: "es2022.error", file: "lib.es2022.error.d.ts"},
	{name: "es2022.intl", file: "lib.es2022.intl.d.ts"},
	{name: "es2022.object", file: "lib.es2022.object.d.ts"},
	{name: "es2022.string", file: "lib.es2022.string.d.ts"},
	{name: "es2022.regexp", file: "lib.es2022.regexp.d.ts"},
	{name: "es2023.array", file: "lib.es2023.array.d.ts"},
	{name: "es2023.collection", file: "lib.es2023.collection.d.ts"},
	{name: "es2023.intl", file: "lib.es2023.intl.d.ts"},
	{name: "es2024.arraybuffer", file: "lib.es2024.arraybuffer.d.ts"},
	{name: "es2024.collection", file: "lib.es2024.collection.d.ts"},
	{name: "es2024.object", file: "lib.es2024.object.d.ts"},
	{name: "es2024.promise", file: "lib.es2024.promise.d.ts"},
	{name: "es2024.regexp", file: "lib.es2024.regexp.d.ts"},
	{name: "es2024.sharedmemory", file: "lib.es2024.sharedmemory.d.ts"},
	{name: "es2024.string", file: "lib.es2024.string.d.ts"},
	{name: "es2025.collection", file: "lib.es2025.collection.d.ts"},
	{name: "es2025.float16", file: "lib.es2025.float16.d.ts"},
	{name: "es2025.intl", file: "lib.es2025.intl.d.ts"},
	{name: "es2025.iterator", file: "lib.es2025.iterator.d.ts"},
	{name: "es2025.promise", file: "lib.es2025.promise.d.ts"},
	{name: "es2025.regexp", file: "lib.es2025.regexp.d.ts"},
	{name: "esnext.asynciterable", file: "lib.es2018.asynciterable.d.ts"},
	{name: "esnext.symbol", file: "lib.es2019.symbol.d.ts"},
	{name: "esnext.bigint", file: "lib.es2020.bigint.d.ts"},
	{name: "esnext.weakref", file: "lib.es2021.weakref.d.ts"},
	{name: "esnext.object", file: "lib.es2024.object.d.ts"},
	{name: "esnext.regexp", file: "lib.es2024.regexp.d.ts"},
	{name: "esnext.string", file: "lib.es2024.string.d.ts"},
	{name: "esnext.float16", file: "lib.es2025.float16.d.ts"},
	{name: "esnext.iterator", file: "lib.es2025.iterator.d.ts"},
	{name: "esnext.promise", file: "lib.es2025.promise.d.ts"},
	{name: "esnext.array", file: "lib.esnext.array.d.ts"},
	{name: "esnext.collection", file: "lib.esnext.collection.d.ts"},
	{name: "esnext.date", file: "lib.esnext.date.d.ts"},
	{name: "esnext.decorators", file: "lib.esnext.decorators.d.ts"},
	{name: "esnext.disposable", file: "lib.esnext.disposable.d.ts"},
	{name: "esnext.error", file: "lib.esnext.error.d.ts"},
	{name: "esnext.intl", file: "lib.esnext.intl.d.ts"},
	{name: "esnext.sharedmemory", file: "lib.esnext.sharedmemory.d.ts"},
	{name: "esnext.temporal", file: "lib.esnext.temporal.d.ts"},
	{name: "esnext.typedarrays", file: "lib.esnext.typedarrays.d.ts"},
	{name: "decorators", file: "lib.decorators.d.ts"},
	{name: "decorators.legacy", file: "lib.decorators.legacy.d.ts"},
}

// defaultLibraries maps each target of TypeScript 7.0.2 onto the file of
// its default library, as targetToLibMap of typescript-go 7.0.2 does. That
// map has no ES5, so ES5 has the default library lib.d.ts.
var defaultLibraries = map[string]string{
	"es5":    defaultLibraryFile,
	"es6":    es6LibraryFile,
	"es2015": es6LibraryFile,
	"es2016": "lib.es2016.full.d.ts",
	"es2017": "lib.es2017.full.d.ts",
	"es2018": "lib.es2018.full.d.ts",
	"es2019": "lib.es2019.full.d.ts",
	"es2020": "lib.es2020.full.d.ts",
	"es2021": "lib.es2021.full.d.ts",
	"es2022": "lib.es2022.full.d.ts",
	"es2023": "lib.es2023.full.d.ts",
	"es2024": "lib.es2024.full.d.ts",
	"es2025": "lib.es2025.full.d.ts",
	"esnext": "lib.esnext.full.d.ts",
}

// The parts of the file of a library: the prefix and the suffix of its
// name, the two files that typescript-go loads before every other, and the
// target that a project without one has, which is ES2025 in TypeScript
// 7.0.2.
const (
	libraryPrefix      = "lib."
	librarySuffix      = ".d.ts"
	defaultLibraryFile = "lib.d.ts"
	es6LibraryFile     = "lib.es6.d.ts"
	defaultTarget      = "es2025"
)

// libraryStoreSep separates the key of a library from a file inside the
// store of the library.
const libraryStoreSep = "/"

// fragmentSep separates a file of the library store from the global name
// whose statements its fragment has, as in lib.es5.d.ts#Error.
const fragmentSep = "#"

// declarations returns the statements of [library.Index] by global name
// and then by file, each list in the order of its file. The store serves
// the fragment of a name from it, the dependency round places the unit of
// a name from it, and the lowering records a use of a name that it
// contains. It builds the map once for the process. Any number of
// goroutines may read the map, and none writes it.
var declarations = sync.OnceValue(func() map[string]map[string][]library.Statement {
	out := map[string]map[string][]library.Statement{}
	for file, entry := range library.Index {
		for _, s := range entry.Statements {
			if out[s.Name] == nil {
				out[s.Name] = map[string][]library.Statement{}
			}
			out[s.Name][file] = append(out[s.Name][file], s)
		}
	}
	return out
})

// sources contains the bytes of each embedded file of the library that
// [sourceOf] read, by the name of the file.
var sources = struct {
	sync.Mutex
	files map[string][]byte
}{files: map[string][]byte{}}

// sourceOf returns the bytes of an embedded file of the library. It reads
// each file once for the process, and every caller shares the bytes, which
// no caller writes. Any number of goroutines may call it.
//
// Error modes: sourceOf returns the error of the embed for a file that the
// release does not have, and for the directory of the library.
func sourceOf(file string) ([]byte, error) {
	sources.Lock()
	defer sources.Unlock()
	if src, read := sources.files[file]; read {
		return src, nil
	}
	src, err := fs.ReadFile(library.Files, path.Join(library.Dir, file))
	if err != nil {
		return nil, err
	}
	sources.files[file] = src
	return src, nil
}

// keys contains the files of each library that [keyFiles] listed, by the
// key of the library.
var keys = struct {
	sync.Mutex
	files map[string][]string
}{files: map[string][]string{}}

// keyFiles returns the files of the library of a key, in the order of
// [sortLibrary]. The files of a library are its root files, which the key
// lists, and each file that a lib directive at the head of one of its files
// refers to, followed to the end. A directive with a name that TypeScript
// 7.0.2 does not know does not add a file. keyFiles reads the files through
// [sourceOf] and lists the files of each key once for the process. Any
// number of goroutines may call it, and no caller writes the list.
//
// Error modes: keyFiles returns the error of [sourceOf] for a root that the
// release does not have.
func keyFiles(key string) ([]string, error) {
	keys.Lock()
	defer keys.Unlock()
	if files, listed := keys.files[key]; listed {
		return files, nil
	}
	files := strings.Split(key, typescript.LibraryKeySep)
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		seen[file] = true
	}
	for i := 0; i < len(files); i++ {
		src, err := sourceOf(files[i])
		if err != nil {
			return nil, err
		}
		for _, ref := range libReferences(files[i], src) {
			//dokimi:mutate-skip lcr-right: each directive of the release refers to a library of the table of names
			if file, known := fileOfLibrary(ref.name); known && !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	sortLibrary(files)
	keys.files[key] = files
	return files, nil
}

// fileOfLibrary returns the declaration file of the library that an entry
// of the lib option or a reference directive names: the file of the name,
// or the name itself where it is the file of a library, without regard to
// case, as GetLibFileName of typescript-go 7.0.2 reads it. It reports false
// for a name that TypeScript 7.0.2 does not know. It allocates the lowered
// name where the name has an upper-case letter.
func fileOfLibrary(name string) (string, bool) {
	name = strings.ToLower(name)
	for _, l := range libraryFiles {
		if l.name == name || l.file == name {
			return l.file, true
		}
	}
	return "", false
}

// rootsOf returns the root files of the library that the options of a
// tsconfig chain select, sorted and without repeats. noLib selects none. A
// lib list selects the file of each entry, and an empty list selects none.
// Without either, the default library of the target is the root. rootsOf
// also returns the entries of lib and the target that TypeScript 7.0.2 does
// not know. An entry that it does not know selects no file, and a target
// that it does not know is ES2025.
func rootsOf(opts libraryOptions) (roots, unknownLib []string, unknownTarget string) {
	switch {
	case opts.noLib != nil && *opts.noLib:
		return nil, nil, ""
	case opts.lib != nil:
		for _, entry := range *opts.lib {
			file, known := fileOfLibrary(entry)
			if !known {
				unknownLib = append(unknownLib, entry)
				continue
			}
			roots = append(roots, file)
		}
		slices.Sort(roots)
		return slices.Compact(roots), unknownLib, ""
	}
	target := defaultTarget
	if opts.target != nil {
		target = strings.ToLower(*opts.target)
	}
	file, known := defaultLibraries[target]
	if !known {
		unknownTarget = *opts.target
		file = defaultLibraries[defaultTarget]
	}
	return []string{file}, nil, unknownTarget
}

// libraryPath returns the key and the file of a path inside the store of
// the library, as lib.es2024.d.ts and lib.es5.d.ts for
// typescript-lib://lib.es2024.d.ts/lib.es5.d.ts. A path of the key alone has
// no file. libraryPath reports false for a path of another store and for a
// workspace path. It allocates nothing.
func libraryPath(p string) (key, file string, ok bool) {
	store, inner, qualified := plugin.CutStorePath(p)
	if !qualified || store != typescript.LibraryStore {
		return "", "", false
	}
	key, file, _ = strings.Cut(inner, libraryStoreSep)
	return key, file, true
}

// loadOrder returns the position of a library file in the order in which
// typescript-go 7.0.2 loads library files: lib.d.ts and lib.es6.d.ts first,
// then the file of each name in the order of [libraryFiles], then every
// other file. It allocates nothing.
func loadOrder(file string) int {
	if file == defaultLibraryFile || file == es6LibraryFile {
		return 0
	}
	name := strings.TrimSuffix(strings.TrimPrefix(file, libraryPrefix), librarySuffix)
	if at := slices.IndexFunc(libraryFiles, func(l libraryFile) bool { return l.name == name }); at >= 0 {
		return at + 1
	}
	return len(libraryFiles) + 2
}

// sortLibrary sorts the files of a library into the order in which
// typescript-go 7.0.2 loads them, [loadOrder], and files of one position by
// name. It allocates nothing.
func sortLibrary(files []string) {
	slices.SortFunc(files, func(a, b string) int {
		return cmp.Or(cmp.Compare(loadOrder(a), loadOrder(b)), strings.Compare(a, b))
	})
}
