// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The fake ct.sym the round cases read: java.lang for release 8 and for
// 9 to 25, java.util for 25 alone, java.sql for 11 to 25, a module's
// declaration, and a file beside the release directories, one inside a
// release directory and one inside a package directory.
const (
	lang8       = "8/java.base/java/lang/Object.sig"
	lang9To25   = "9ABCDEFGHIJKLMNOP/java.base/java/lang/Object.sig"
	utilList    = "P/java.base/java/util/List.sig"
	utilMap     = "P/java.base/java/util/Map.sig"
	sqlConn     = "BCDEFGHIJKLMNOP/java.sql/java/sql/Connection.sig"
	moduleSig   = "P/java.base/module-info.sig"
	rootFile    = "README"
	releaseFile = "P/README"
	packageFile = "P/java.base/java/util/README"
	sqlPackage  = "java/sql"
	mapNeed     = "java/util/Map"
	appNeed     = "com/acme/app"
)

// The java.lang entries of the ct.sym files whose newest release the
// ranking cases read: releases 8 and 9, release 10 alone, release 34
// alone, and releases 34 and 35.
const (
	lang89 = "89/java.base/java/lang/Object.sig"
	langA  = "A/java.base/java/lang/Object.sig"
	langY  = "Y/java.base/java/lang/Object.sig"
	langYZ = "YZ/java.base/java/lang/Object.sig"
)

// The classpath library the round cases place: its coordinates, its
// paths in the Maven and the Gradle store, the bytes of its JAR, and an
// entry of its Gradle directory that is no digest's directory.
const (
	libCoordinate = "com.acme:lib:1.0"
	mavenJAR      = "com/acme/lib/1.0/lib-1.0.jar"
	mavenRecord   = mavenJAR + ".sha1"
	gradleVersion = "com.acme/lib/1.0"
	jarName       = "lib-1.0.jar"
	jarBytes      = "jar"
	gradleNotes   = "!notes.txt"
)

// The releases the round cases name: one ct.sym does not list, one older
// than any ct.sym names, the newest and the oldest that a letter names,
// and the ones past the letters.
const (
	unlisted  = 26
	tooOld    = 7
	release8  = 8
	release10 = 10
	release11 = 11
	newest    = 35
	pastLast  = 36
)

// The needs the placement reports name: a class of the classpath JAR's
// package, a package a multi-release JAR has under version 21 alone, and
// that package's class entry.
const (
	boxNeed   = libPackage + "/" + boxClass
	lateNeed  = "com/acme/late"
	lateEntry = "META-INF/versions/21/" + lateNeed + "/Late.class"
)

// The reasons a round reports a need placed nowhere with: the JDK store
// provided, and not.
const (
	noPackageReason = "neither the " + frontend.JDKStore + " store nor a classpath library has its package"
	noJDKReason     = "the load provides no " + frontend.JDKStore + " store, and no classpath library has its package"
)

// The texts of the errors a round returns, as each refusal case expects
// them.
const (
	malformedText  = "is no group:artifact:version"
	neitherText    = "neither the m2 store nor the gradle store"
	noRecordText   = "has no SHA-1 record"
	emptyRecord    = `records ""`
	otherRecord    = "records"
	otherDirText   = "its directory names"
	noReleasesText = "ct.sym lists no release"
)

// storeTree is a test workspace with named stores beside it.
type storeTree struct {
	fstest.MapFS
	stores map[string]fs.FS
}

// Store returns one of the tree's stores.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, provided := t.stores[name]
	return s, provided
}

// roundReader is a dependency round's door over a test workspace and its
// stores.
type roundReader struct {
	tree storeTree
}

// Read returns one file's bytes, from the workspace or a store.
func (r roundReader) Read(p string) ([]byte, error) { return plugin.ReadFile(r.tree, p) }

// ReadDir returns one directory's entries, from the workspace or a
// store.
func (r roundReader) ReadDir(p string) ([]fs.DirEntry, error) { return plugin.ReadDir(r.tree, p) }

// failingFS is a store whose one path fails to open with an error that
// is not a missing file's.
type failingFS struct {
	fs.FS
	fail string
}

// Open opens a path of the store, and fails for the failing one.
func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.fail {
		return nil, errBroken
	}
	return f.FS.Open(name)
}

// errBroken is the error a failing store's path fails with.
var errBroken = errors.New("broken store")

// The dependency round places the JDK packages the needs name and the
// classpath's libraries, so each source, each refusal and the
// placements that yield nothing are pinned over fixture stores.
func TestDepend(t *testing.T) {
	t.Parallel()

	t.Run("dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns java/lang in the first round", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, nil, jdkStores(), 1), [][]string{{inJDK(lang9To25)}}, "the newest release's")
		})

		t.Run("returns no java/lang after the first round", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, nil, jdkStores(), 2), "the first round loaded it")
		})

		t.Run("returns the JDK package a need names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, nil, jdkStores(), 2, utilPackage), [][]string{{inJDK(utilList), inJDK(utilMap)}},
				"its class files in path order, and no file that is no class file")
		})

		t.Run("returns the package of the class a need names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, nil, jdkStores(), 2, mapNeed), [][]string{{inJDK(utilList), inJDK(utilMap)}},
				"a static import names Map")
		})

		t.Run("returns a package once for two needs that name it", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, placed(t, nil, jdkStores(), 2, utilPackage, mapNeed), 1, "java/util once")
		})

		t.Run("returns no unit for a need ct.sym has no package for", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, nil, jdkStores(), 2, appNeed), "a package of no library")
		})

		t.Run("returns the units of the release the options name", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: release11}, jdkStores(), 1, utilPackage, sqlPackage)
			assert.Equal(t, got, [][]string{{inJDK(lang9To25)}, {inJDK(sqlConn)}}, "java/util is 25's alone")
		})

		t.Run("returns release 8's units for release 8", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: release8}, jdkStores(), 1)
			assert.Equal(t, got, [][]string{{inJDK(lang8)}}, "the digit names release 8")
		})

		t.Run("returns release 10's units past a file whose name has its letter", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: release10}, jdkStores(), 1)
			assert.Equal(t, got, [][]string{{inJDK(lang9To25)}}, "A names release 10, and README is no directory")
		})

		t.Run("returns the newest release's units for the newest release", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: release25}, jdkStores(), 1)
			assert.Equal(t, got, [][]string{{inJDK(lang9To25)}}, "P names release 25")
		})

		ranked := []struct {
			name  string
			store fstest.MapFS
			want  string
		}{
			{
				name:  "returns release 9's units for release 0 of a ct.sym that lists 9 last",
				store: fstest.MapFS{lang8: {}, lang89: {}}, want: lang89,
			},
			{
				name:  "returns release 10's units for release 0 of a ct.sym that lists 10 last",
				store: fstest.MapFS{lang89: {}, langA: {}}, want: langA,
			},
			{
				name:  "returns release 35's units for release 0 of a ct.sym that lists 35 last",
				store: fstest.MapFS{langY: {}, langYZ: {}}, want: langYZ,
			},
		}
		for _, tt := range ranked {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := placed(t, nil, map[string]fs.FS{frontend.JDKStore: tt.store}, 1)
				assert.Equal(t, got, [][]string{{inJDK(tt.want)}}, "the newest release, digits before letters")
			})
		}

		t.Run("returns release 35's units for release 35", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: newest}, map[string]fs.FS{
				frontend.JDKStore: fstest.MapFS{langY: {}, langYZ: {}},
			}, 1)
			assert.Equal(t, got, [][]string{{inJDK(langYZ)}}, "Z names release 35")
		})

		t.Run("returns a library's JAR from the Maven store with its SHA-1 record shared", func(t *testing.T) {
			t.Parallel()

			stores := map[string]fs.FS{frontend.MavenStore: mavenStore(sha1Hex(jarBytes))}
			units := rounds(t, &frontend.Options{Classpath: []string{libCoordinate}}, stores, 1)
			assert.Equal(t, units, [][]plugin.SourceRef{{{
				Path: inMaven(mavenJAR), Shared: []string{inMaven(mavenRecord)},
			}}}, "the JAR and its record")
		})

		t.Run("returns a library's JAR whose SHA-1 record states the digest in upper case", func(t *testing.T) {
			t.Parallel()

			stores := map[string]fs.FS{frontend.MavenStore: mavenStore(strings.ToUpper(sha1Hex(jarBytes)))}
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, stores, 1)
			assert.Equal(t, got, [][]string{{inMaven(mavenJAR)}}, "hexadecimal digits compare without case")
		})

		t.Run("returns a library's JAR from the Gradle store where the Maven store lacks it", func(t *testing.T) {
			t.Parallel()

			stores := map[string]fs.FS{frontend.GradleStore: gradleStore(jarBytes, sha1Hex(jarBytes))}
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, stores, 1)
			assert.Equal(t, got, [][]string{{inGradle(sha1Hex(jarBytes))}}, "the JAR in its digest's directory")
		})

		t.Run("returns a Gradle JAR whose directory drops its digest's leading zeros", func(t *testing.T) {
			t.Parallel()

			data := zeroLedJAR()
			dir := strings.TrimLeft(sha1Hex(data), "0")
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: gradleStore(data, dir),
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(dir)}}, "Gradle drops the zeros from the name")
		})

		t.Run("returns a Gradle JAR whose directory keeps its digest's leading zeros", func(t *testing.T) {
			t.Parallel()

			data := zeroLedJAR()
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: gradleStore(data, sha1Hex(data)),
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(sha1Hex(data))}}, "the full digest names it as well")
		})

		t.Run("returns no JDK unit for a load without the JDK store", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, nil, map[string]fs.FS{}, 1, utilPackage), "the source is off")
		})

		refused := []struct {
			name   string
			opts   *frontend.Options
			stores map[string]fs.FS
			want   string
		}{
			{
				name: "returns an error for a library without three parts",
				opts: &frontend.Options{Classpath: []string{"com.acme:lib"}}, stores: jdkStores(), want: malformedText,
			},
			{
				name: "returns an error for a library with an empty part",
				opts: &frontend.Options{Classpath: []string{"com.acme::1.0"}}, stores: jdkStores(), want: malformedText,
			},
			{
				name: "returns an error for a library neither store has",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: jdkStores(), want: neitherText,
			},
			{
				name: "returns an error for a library whose Gradle directory neither store has",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: fstest.MapFS{
					"org/other/1.0/0000/other-1.0.jar": {},
				}},
				want: neitherText,
			},
			{
				name:   "returns an error for a Maven JAR whose record states another digest",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.MavenStore: mavenStore(sha1Hex("other"))}, want: otherRecord,
			},
			{
				name: "returns an error for a Maven JAR whose record is empty",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.MavenStore: fstest.MapFS{
					mavenJAR: {Data: []byte(jarBytes)}, mavenRecord: {},
				}},
				want: emptyRecord,
			},
			{
				name: "returns an error for a Maven JAR without a record",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.MavenStore: fstest.MapFS{
					mavenJAR: {Data: []byte(jarBytes)},
				}},
				want: noRecordText,
			},
			{
				name:   "returns an error for a Gradle JAR whose directory names another digest",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: gradleStore(jarBytes, sha1Hex("other"))},
				want:   otherDirText,
			},
			{
				name: "returns an error for a Gradle version directory without the JAR",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: fstest.MapFS{
					gradleVersion + "/0000/lib-1.0.pom": {},
				}},
				want: neitherText,
			},
			{
				name: "returns an error for a release ct.sym does not list", opts: &frontend.Options{Release: unlisted},
				stores: jdkStores(), want: fmt.Sprintf("lists no release %d", unlisted),
			},
			{
				name: "returns an error for a release past the newest ct.sym lists",
				opts: &frontend.Options{Release: newest}, stores: jdkStores(),
				want: fmt.Sprintf("lists no release %d", newest),
			},
			{
				name: "returns an error for a release older than ct.sym can name",
				opts: &frontend.Options{Release: tooOld}, stores: jdkStores(),
				want: fmt.Sprintf("names no release %d", tooOld),
			},
			{
				name: "returns an error for a release newer than ct.sym can name",
				opts: &frontend.Options{Release: pastLast}, stores: jdkStores(),
				want: fmt.Sprintf("names no release %d", pastLast),
			},
			{
				name: "returns an error for a ct.sym that lists no release", stores: map[string]fs.FS{
					frontend.JDKStore: fstest.MapFS{rootFile: {}},
				},
				want: noReleasesText,
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := runRound(tt.opts, tt.stores, 1)
				assert.HasError(t, err, "the round fails")
				assert.True(t, strings.Contains(err.Error(), tt.want), "for its own reason")
			})
		}

		broken := []struct {
			name   string
			opts   *frontend.Options
			stores map[string]fs.FS
		}{
			{
				name:   "returns the store's error for a Maven JAR that does not read",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.MavenStore: failingFS{mavenStore(sha1Hex(jarBytes)), mavenJAR}},
			},
			{
				name: "returns the store's error for a Gradle directory that does not list",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: failingFS{
					gradleStore(jarBytes, sha1Hex(jarBytes)), gradleVersion,
				}},
			},
			{
				name: "returns the store's error for a Gradle JAR that does not read",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: failingFS{
					gradleStore(jarBytes, sha1Hex(jarBytes)), path.Join(gradleVersion, sha1Hex(jarBytes), jarName),
				}},
			},
			{
				name:   "returns the store's error for a ct.sym whose root does not list",
				stores: map[string]fs.FS{frontend.JDKStore: failingFS{jdkStore(), "."}},
			},
			{
				name:   "returns the store's error for a release directory that does not list",
				stores: map[string]fs.FS{frontend.JDKStore: failingFS{jdkStore(), "P"}},
			},
			{
				name:   "returns the store's error for a package directory that does not list",
				stores: map[string]fs.FS{frontend.JDKStore: failingFS{jdkStore(), "P/java.base/java"}},
			},
		}
		for _, tt := range broken {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := runRound(tt.opts, tt.stores, 1)
				assert.ErrorIs(t, err, errBroken, "the store's own failure")
			})
		}

		t.Run("skips a Gradle entry that is no directory", func(t *testing.T) {
			t.Parallel()

			cache := t.TempDir()
			writeFile(t, cache, path.Join(gradleVersion, sha1Hex(jarBytes), jarName), []byte(jarBytes))
			writeFile(t, cache, path.Join(gradleVersion, gradleNotes), nil)
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: os.DirFS(cache),
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(sha1Hex(jarBytes))}},
				"a file contains no JAR, and reads as no directory")
		})

		t.Run("skips a Gradle digest directory without the JAR", func(t *testing.T) {
			t.Parallel()

			store := gradleStore(jarBytes, sha1Hex(jarBytes))
			store[gradleVersion+"/0000/lib-1.0.pom"] = &fstest.MapFile{}
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: store,
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(sha1Hex(jarBytes))}}, "the directory of the POM is passed")
		})

		t.Run("returns the context's error for a done context", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			dependent, _ := frontend.New(nil).(plugin.Dependent)
			_, err := dependent.Dependencies(ctx, &plugin.DependencyRound{Number: 1},
				roundReader{storeTree{fstest.MapFS{}, jdkStores()}})
			assert.ErrorIs(t, err, context.Canceled, "a cancelled round walks nothing")
		})

		lateJAR := jarOf(t, multiRelease, map[string][]byte{lateEntry: nil})
		unplaced := []struct {
			name   string
			opts   *frontend.Options
			stores map[string]fs.FS
			number int
			need   string
			want   []plugin.Unplaced
		}{
			{
				name: "reports a need neither ct.sym nor a classpath library has placed nowhere", stores: jdkStores(),
				number: 2, need: appNeed, want: []plugin.Unplaced{{Path: appNeed, Reason: noPackageReason}},
			},
			{
				name: "reports a need placed nowhere for a load without the JDK store", stores: map[string]fs.FS{},
				number: 1, need: utilPackage, want: []plugin.Unplaced{{Path: utilPackage, Reason: noJDKReason}},
			},
			{
				name: "reports nothing for a need of a JDK class", stores: jdkStores(), number: 2, need: mapNeed,
			},
			{
				name: "reports nothing for a need of a classpath library's package",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: libStores(t, mustRead(t, libJAR)),
				number: 1, need: libPackage,
			},
			{
				name: "reports nothing for a need of a class in a classpath library",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: libStores(t, mustRead(t, libJAR)),
				number: 1, need: boxNeed,
			},
			{
				name: "reports a need placed nowhere where the classpath JAR does not open",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: libStores(t, []byte(jarBytes)),
				number: 1, need: libPackage, want: []plugin.Unplaced{{Path: libPackage, Reason: noPackageReason}},
			},
			{
				name:   "reports nothing for a need of a multi-release JAR's versioned package",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}, Release: release21},
				stores: libStores(t, lateJAR), number: 1, need: lateNeed,
			},
			{
				name:   "reports a need of a versioned package placed nowhere for an older release",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}, Release: release17},
				stores: libStores(t, lateJAR), number: 1, need: lateNeed,
				want: []plugin.Unplaced{{Path: lateNeed, Reason: noPackageReason}},
			},
		}
		for _, tt := range unplaced {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				round := &plugin.DependencyRound{Number: tt.number, Needs: []plugin.Need{{Path: tt.need}}}
				dependent, _ := frontend.New(tt.opts).(plugin.Dependent)
				_, err := dependent.Dependencies(context.Background(), round,
					roundReader{storeTree{fstest.MapFS{}, tt.stores}})
				assert.NoError(t, err, "the round places what it can")
				assert.Equal(t, round.Unplaced(), tt.want, "the needs the round reports and why")
			})
		}
	})
}

// libStores returns the fake ct.sym beside a Maven store with the
// library's JAR of some bytes and their SHA-1 record.
func libStores(tb assert.TB, data []byte) map[string]fs.FS {
	tb.Helper()

	stores := jdkStores()
	stores[frontend.MavenStore] = fstest.MapFS{
		mavenJAR:    {Data: data},
		mavenRecord: {Data: []byte(sha1Hex(string(data)) + "  " + jarName + "\n")},
	}
	return stores
}

// jdkStore returns the fake ct.sym.
func jdkStore() fstest.MapFS {
	return fstest.MapFS{
		lang8: {}, lang9To25: {}, utilList: {}, utilMap: {}, sqlConn: {}, moduleSig: {}, rootFile: {}, releaseFile: {},
		packageFile: {},
	}
}

// jdkStores returns the stores of a load with the fake ct.sym alone.
func jdkStores() map[string]fs.FS { return map[string]fs.FS{frontend.JDKStore: jdkStore()} }

// mavenStore returns a Maven store with the library's JAR, beside a
// record of a digest and the JAR's name, as Maven writes it.
func mavenStore(record string) fstest.MapFS {
	return fstest.MapFS{
		mavenJAR:    {Data: []byte(jarBytes)},
		mavenRecord: {Data: []byte(record + "  " + jarName + "\n")},
	}
}

// gradleStore returns a Gradle store with the library's JAR of some
// bytes in a directory of a name.
func gradleStore(data, dir string) fstest.MapFS {
	return fstest.MapFS{path.Join(gradleVersion, dir, jarName): {Data: []byte(data)}}
}

// zeroLedJAR returns the first bytes of the form "jar N" whose SHA-1
// digest opens with a zero.
func zeroLedJAR() string {
	for n := 0; ; n++ {
		if data := fmt.Sprintf("%s %d", jarBytes, n); strings.HasPrefix(sha1Hex(data), "0") {
			return data
		}
	}
}

// sha1Hex returns the SHA-1 digest of a text in lowercase hexadecimal.
func sha1Hex(data string) string {
	sum := sha1.Sum([]byte(data))
	return hex.EncodeToString(sum[:])
}

// runRound runs a round of a number through a Java frontend of options,
// over an empty workspace and stores, for the needs given.
func runRound(
	opts *frontend.Options, stores map[string]fs.FS, number int, needs ...string,
) ([][]plugin.SourceRef, error) {
	round := &plugin.DependencyRound{Number: number}
	for _, need := range needs {
		round.Needs = append(round.Needs, plugin.Need{Path: need})
	}
	dependent, _ := frontend.New(opts).(plugin.Dependent)
	return dependent.Dependencies(context.Background(), round, roundReader{storeTree{fstest.MapFS{}, stores}})
}

// rounds runs a round that the case states succeeds and returns its
// units.
func rounds(
	tb assert.TB, opts *frontend.Options, stores map[string]fs.FS, number int, needs ...string,
) [][]plugin.SourceRef {
	tb.Helper()

	units, err := runRound(opts, stores, number, needs...)
	assert.NoError(tb, err, "the round places its needs")
	return units
}

// placed runs a round that the case states succeeds and returns the
// member paths of each unit.
func placed(tb assert.TB, opts *frontend.Options, stores map[string]fs.FS, number int, needs ...string) [][]string {
	tb.Helper()

	var out [][]string
	for _, unit := range rounds(tb, opts, stores, number, needs...) {
		var members []string
		for _, ref := range unit {
			members = append(members, ref.Path)
		}
		out = append(out, members)
	}
	return out
}

// inJDK returns the qualified path of a ct.sym entry.
func inJDK(p string) string { return plugin.StorePath(frontend.JDKStore, p) }

// inMaven returns the qualified path of a Maven store file.
func inMaven(p string) string { return plugin.StorePath(frontend.MavenStore, p) }

// inGradle returns the qualified path of the library's JAR in a Gradle
// digest directory.
func inGradle(dir string) string {
	return plugin.StorePath(frontend.GradleStore, path.Join(gradleVersion, dir, jarName))
}
