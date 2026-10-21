// Copyright Dokimasia B.V. 2026
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

// The entries of the fake ct.sym that the round cases read: java.lang for
// release 8 and for 9 to 25, java.util for 25 alone, java.sql for 11 to
// 25, the declaration of a module, and three files that are not class
// files: one beside the release directories, one inside a release
// directory and one inside a package directory.
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

// The classpath library that the round cases place: its coordinates, its
// paths in the Maven and the Gradle store, the bytes of its JAR, and an
// entry of its Gradle directory that is not the directory of a digest.
const (
	libCoordinate = "com.acme:lib:1.0"
	mavenJAR      = "com/acme/lib/1.0/lib-1.0.jar"
	mavenRecord   = mavenJAR + ".sha1"
	gradleVersion = "com.acme/lib/1.0"
	jarName       = "lib-1.0.jar"
	jarBytes      = "jar"
	gradleNotes   = "!notes.txt"
)

// The releases of the round cases: one that ct.sym does not list, one
// older than any release that ct.sym names, the newest and the oldest
// release that a letter names, and one past the letters.
const (
	unlisted  = 26
	tooOld    = 7
	release8  = 8
	release10 = 10
	release11 = 11
	newest    = 35
	pastLast  = 36
)

// The needs of the placement reports: a class in the package of the
// classpath JAR, a package that a multi-release JAR has under version 21
// alone, and the class entry of that package.
const (
	boxNeed   = libPackage + "/" + boxClass
	lateNeed  = "com/acme/late"
	lateEntry = "META-INF/versions/21/" + lateNeed + "/Late.class"
)

// The outputs of the brand in the workspace that the placement reports
// read: a class in the package of appNeed below a source root, the same
// class at the root of the workspace, the class in a directory whose last
// names end with the package path inside a name, and the need of a static
// import from the class.
const (
	appOutput     = "src/test/java/com/acme/app/AppStub.java"
	rootOutput    = "com/acme/app/AppStub.java"
	partialOutput = "src/test/java/xcom/acme/app/AppStub.java"
	stubNeed      = appNeed + "/AppStub"
)

// The reasons with which a round reports a need as placed nowhere, with
// the JDK store provided and without it.
const (
	noPackageReason = "neither the " + frontend.JDKStore + " store nor a classpath library has its package"
	noJDKReason     = "the load provides no " + frontend.JDKStore + " store, and no classpath library has its package"
)

// The texts of the errors that a round returns, as each error case expects
// them.
const (
	malformedText  = "is not group:artifact:version"
	neitherText    = "neither the m2 store nor the gradle store"
	noRecordText   = "has no SHA-1 record"
	emptyRecord    = `records ""`
	otherRecord    = "records"
	otherDirText   = "the name of its directory is"
	noReleasesText = "ct.sym lists no release"
)

// storeTree is a test workspace with named stores beside it.
type storeTree struct {
	fstest.MapFS
	stores map[string]fs.FS
}

// Store returns one store of the tree.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, provided := t.stores[name]
	return s, provided
}

// roundReader is the door of a dependency round over a test workspace and
// its stores.
type roundReader struct {
	tree storeTree
}

// Read returns the bytes of one file from the workspace or a store.
func (r roundReader) Read(p string) ([]byte, error) { return plugin.ReadFile(r.tree, p) }

// ReadDir returns the entries of one directory from the workspace or a
// store.
func (r roundReader) ReadDir(p string) ([]fs.DirEntry, error) { return plugin.ReadDir(r.tree, p) }

// failingFS is a store in which one path fails to open with an error that
// differs from the error of a missing file.
type failingFS struct {
	fs.FS
	fail string
}

// Open opens a path of the store, and returns errBroken for the failing
// path.
func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.fail {
		return nil, errBroken
	}
	return f.FS.Open(name)
}

// errBroken is the error of the failing path of a failing store.
var errBroken = errors.New("broken store")

// The dependency round places the JDK packages of the needs and the
// libraries of the classpath. The cases pin each source, each error and
// each placement that yields nothing over fixture stores.
func TestDepend(t *testing.T) {
	t.Parallel()

	t.Run("dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns java/lang in the first round", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, nil, jdkStores(), 1), [][]string{{inJDK(lang9To25)}},
				"the unit of the newest release")
		})

		t.Run("returns no java/lang after the first round", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, nil, jdkStores(), 2), "the first round loaded it")
		})

		t.Run("returns the JDK package of a need", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, nil, jdkStores(), 2, utilPackage), [][]string{{inJDK(utilList), inJDK(utilMap)}},
				"its class files in path order, and no file that is not a class file")
		})

		t.Run("returns the package of the class of a need", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, nil, jdkStores(), 2, mapNeed), [][]string{{inJDK(utilList), inJDK(utilMap)}},
				"a static import names Map")
		})

		t.Run("returns a package once for two needs of the package", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, placed(t, nil, jdkStores(), 2, utilPackage, mapNeed), 1, "java/util once")
		})

		t.Run("returns no unit for a need whose package ct.sym does not have", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, nil, jdkStores(), 2, appNeed), "a package of no library")
		})

		t.Run("returns the units of the release of the options", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: release11}, jdkStores(), 1, utilPackage, sqlPackage)
			assert.Equal(t, got, [][]string{{inJDK(lang9To25)}, {inJDK(sqlConn)}}, "java/util is in release 25 alone")
		})

		t.Run("returns the units of release 8 for release 8", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: release8}, jdkStores(), 1)
			assert.Equal(t, got, [][]string{{inJDK(lang8)}}, "the digit names release 8")
		})

		t.Run("returns the units of release 10 past a file whose name contains its letter", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: release10}, jdkStores(), 1)
			assert.Equal(t, got, [][]string{{inJDK(lang9To25)}}, "A names release 10, and README is not a directory")
		})

		t.Run("returns the units of the newest release for the newest release", func(t *testing.T) {
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
				name:  "returns the units of release 9 for release 0 of a ct.sym that lists 9 last",
				store: fstest.MapFS{lang8: {}, lang89: {}}, want: lang89,
			},
			{
				name:  "returns the units of release 10 for release 0 of a ct.sym that lists 10 last",
				store: fstest.MapFS{lang89: {}, langA: {}}, want: langA,
			},
			{
				name:  "returns the units of release 35 for release 0 of a ct.sym that lists 35 last",
				store: fstest.MapFS{langY: {}, langYZ: {}}, want: langYZ,
			},
		}
		for _, tt := range ranked {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := placed(t, nil, map[string]fs.FS{frontend.JDKStore: tt.store}, 1)
				assert.Equal(t, got, [][]string{{inJDK(tt.want)}}, "the newest release, with digits before letters")
			})
		}

		t.Run("returns the units of release 35 for release 35", func(t *testing.T) {
			t.Parallel()

			got := placed(t, &frontend.Options{Release: newest}, map[string]fs.FS{
				frontend.JDKStore: fstest.MapFS{langY: {}, langYZ: {}},
			}, 1)
			assert.Equal(t, got, [][]string{{inJDK(langYZ)}}, "Z names release 35")
		})

		t.Run("returns the JAR of a library from the Maven store with its SHA-1 record as a shared input",
			func(t *testing.T) {
				t.Parallel()

				stores := map[string]fs.FS{frontend.MavenStore: mavenStore(sha1Hex(jarBytes))}
				units := rounds(t, &frontend.Options{Classpath: []string{libCoordinate}}, stores, 1)
				assert.Equal(t, units, [][]plugin.SourceRef{{{
					Path: inMaven(mavenJAR), Shared: []string{inMaven(mavenRecord)},
				}}}, "the JAR and its record")
			})

		t.Run("returns the JAR of a library whose SHA-1 record contains the digest in upper case", func(t *testing.T) {
			t.Parallel()

			stores := map[string]fs.FS{frontend.MavenStore: mavenStore(strings.ToUpper(sha1Hex(jarBytes)))}
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, stores, 1)
			assert.Equal(t, got, [][]string{{inMaven(mavenJAR)}}, "hexadecimal digits compare without regard to case")
		})

		t.Run("returns the JAR of a library from the Gradle store where the Maven store does not have it",
			func(t *testing.T) {
				t.Parallel()

				stores := map[string]fs.FS{frontend.GradleStore: gradleStore(jarBytes, sha1Hex(jarBytes))}
				got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, stores, 1)
				assert.Equal(t, got, [][]string{{inGradle(sha1Hex(jarBytes))}},
					"the JAR in the directory of its digest")
			})

		t.Run("returns a Gradle JAR in a directory without the leading zeros of its digest", func(t *testing.T) {
			t.Parallel()

			data := zeroLedJAR()
			dir := strings.TrimLeft(sha1Hex(data), "0")
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: gradleStore(data, dir),
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(dir)}}, "Gradle drops the zeros from the name")
		})

		t.Run("returns a Gradle JAR in a directory with the leading zeros of its digest", func(t *testing.T) {
			t.Parallel()

			data := zeroLedJAR()
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: gradleStore(data, sha1Hex(data)),
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(sha1Hex(data))}}, "the full digest is a valid name as well")
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
				name: "returns an error for a library that neither store has",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: jdkStores(), want: neitherText,
			},
			{
				name: "returns an error for a library whose Gradle directory is missing",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: fstest.MapFS{
					"org/other/1.0/0000/other-1.0.jar": {},
				}},
				want: neitherText,
			},
			{
				name:   "returns an error for a Maven JAR whose record contains another digest",
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
				name:   "returns an error for a Gradle JAR in the directory of another digest",
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
				name: "returns an error for a release that ct.sym does not list",
				opts: &frontend.Options{Release: unlisted}, stores: jdkStores(),
				want: fmt.Sprintf("lists no release %d", unlisted),
			},
			{
				name: "returns an error for a release past the newest release that ct.sym lists",
				opts: &frontend.Options{Release: newest}, stores: jdkStores(),
				want: fmt.Sprintf("lists no release %d", newest),
			},
			{
				name:   "returns an error for a release older than every release that ct.sym lists",
				opts:   &frontend.Options{Release: release8},
				stores: map[string]fs.FS{frontend.JDKStore: fstest.MapFS{lang9To25: {}}},
				want:   fmt.Sprintf("lists no release %d", release8),
			},
			{
				name: "returns an error for a release older than the oldest release that ct.sym can name",
				opts: &frontend.Options{Release: tooOld}, stores: jdkStores(),
				want: fmt.Sprintf("names no release %d", tooOld),
			},
			{
				name: "returns an error for a release newer than the newest release that ct.sym can name",
				opts: &frontend.Options{Release: pastLast}, stores: jdkStores(),
				want: fmt.Sprintf("names no release %d", pastLast),
			},
			{
				name: "returns an error for a ct.sym that does not list a release", stores: map[string]fs.FS{
					frontend.JDKStore: fstest.MapFS{rootFile: {}},
				},
				want: noReleasesText,
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := runRound(t, tt.opts, tt.stores, 1)
				assert.HasError(t, err, "the round fails")
				assert.Contains(t, err.Error(), tt.want, "the error states its own reason")
			})
		}

		broken := []struct {
			name   string
			opts   *frontend.Options
			stores map[string]fs.FS
		}{
			{
				name:   "returns the error of the store for a Maven JAR that does not read",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.MavenStore: failingFS{mavenStore(sha1Hex(jarBytes)), mavenJAR}},
			},
			{
				name: "returns the error of the store for a Gradle directory that does not list",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: failingFS{
					gradleStore(jarBytes, sha1Hex(jarBytes)), gradleVersion,
				}},
			},
			{
				name: "returns the error of the store for a Gradle JAR that does not read",
				opts: &frontend.Options{Classpath: []string{libCoordinate}},
				stores: map[string]fs.FS{frontend.GradleStore: failingFS{
					gradleStore(jarBytes, sha1Hex(jarBytes)), path.Join(gradleVersion, sha1Hex(jarBytes), jarName),
				}},
			},
			{
				name:   "returns the error of the store for a ct.sym whose root does not list",
				stores: map[string]fs.FS{frontend.JDKStore: failingFS{jdkStore(), "."}},
			},
			{
				name:   "returns the error of the store for a release directory that does not list",
				stores: map[string]fs.FS{frontend.JDKStore: failingFS{jdkStore(), "P"}},
			},
			{
				name:   "returns the error of the store for a package directory that does not list",
				stores: map[string]fs.FS{frontend.JDKStore: failingFS{jdkStore(), "P/java.base/java"}},
			},
		}
		for _, tt := range broken {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := runRound(t, tt.opts, tt.stores, 1)
				assert.ErrorIs(t, err, errBroken, "the round returns the failure of the store")
			})
		}

		t.Run("skips a Gradle entry that is not a directory", func(t *testing.T) {
			t.Parallel()

			cache := t.TempDir()
			writeFile(t, cache, path.Join(gradleVersion, sha1Hex(jarBytes), jarName), []byte(jarBytes))
			writeFile(t, cache, path.Join(gradleVersion, gradleNotes), nil)
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: os.DirFS(cache),
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(sha1Hex(jarBytes))}},
				"a file contains no JAR and does not read as a directory")
		})

		t.Run("skips a Gradle digest directory without the JAR", func(t *testing.T) {
			t.Parallel()

			store := gradleStore(jarBytes, sha1Hex(jarBytes))
			store[gradleVersion+"/0000/lib-1.0.pom"] = &fstest.MapFile{}
			got := placed(t, &frontend.Options{Classpath: []string{libCoordinate}}, map[string]fs.FS{
				frontend.GradleStore: store,
			}, 1)
			assert.Equal(t, got, [][]string{{inGradle(sha1Hex(jarBytes))}}, "the round passes the directory of the POM")
		})

		t.Run("returns the error of the context for a done context", func(t *testing.T) {
			t.Parallel()

			dependent, _ := frontend.New(nil).(plugin.Dependent)
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := dependent.Dependencies(ctx, &plugin.DependencyRound{Number: 1},
					roundReader{storeTree{fstest.MapFS{}, jdkStores()}})
				return err
			}, "a cancelled round walks nothing")
		})

		lateJAR := jarOf(t, multiRelease, map[string][]byte{lateEntry: nil})
		unplaced := []struct {
			name    string
			opts    *frontend.Options
			stores  map[string]fs.FS
			number  int
			need    string
			outputs []string
			want    []plugin.Unplaced
		}{
			{
				name:   "reports a need as placed nowhere where neither ct.sym nor a classpath library has its package",
				stores: jdkStores(), number: 2, need: appNeed,
				want: []plugin.Unplaced{{Path: appNeed, Reason: noPackageReason}},
			},
			{
				name: "reports a need as placed nowhere for a load without the JDK store", stores: map[string]fs.FS{},
				number: 1, need: utilPackage, want: []plugin.Unplaced{{Path: utilPackage, Reason: noJDKReason}},
			},
			{
				name: "reports nothing for a need of a JDK class", stores: jdkStores(), number: 2, need: mapNeed,
			},
			{
				name: "reports nothing for a need of the package of a classpath library",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: libStores(t, mustRead(t, libJAR)),
				number: 1, need: libPackage,
			},
			{
				name: "reports nothing for a need of a class in a classpath library",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: libStores(t, mustRead(t, libJAR)),
				number: 1, need: boxNeed,
			},
			{
				name: "reports a need as placed nowhere where the classpath JAR does not open",
				opts: &frontend.Options{Classpath: []string{libCoordinate}}, stores: libStores(t, []byte(jarBytes)),
				number: 1, need: libPackage, want: []plugin.Unplaced{{Path: libPackage, Reason: noPackageReason}},
			},
			{
				name:   "reports nothing for a need of a versioned package of a multi-release JAR",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}, Release: release21},
				stores: libStores(t, lateJAR), number: 1, need: lateNeed,
			},
			{
				name:   "reports a need of a versioned package as placed nowhere for an older release",
				opts:   &frontend.Options{Classpath: []string{libCoordinate}, Release: release17},
				stores: libStores(t, lateJAR), number: 1, need: lateNeed,
				want: []plugin.Unplaced{{Path: lateNeed, Reason: noPackageReason}},
			},
			{
				name:   "reports nothing for a need of a package whose directory below a source root has an output",
				stores: jdkStores(), number: 2, need: appNeed, outputs: []string{appOutput},
			},
			{
				name:   "reports nothing for a need of a package whose directory at the root has an output",
				stores: jdkStores(), number: 2, need: appNeed, outputs: []string{rootOutput},
			},
			{
				name:   "reports nothing for a need of a class in a package whose directory has an output",
				stores: jdkStores(), number: 2, need: stubNeed, outputs: []string{appOutput},
			},
			{
				name:   "reports a need as placed nowhere where its package ends an output directory inside a name",
				stores: jdkStores(), number: 2, need: appNeed, outputs: []string{partialOutput},
				want: []plugin.Unplaced{{Path: appNeed, Reason: noPackageReason}},
			},
		}
		for _, tt := range unplaced {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				round := &plugin.DependencyRound{
					Number: tt.number, Needs: []plugin.Need{{Path: tt.need}}, Outputs: tt.outputs,
				}
				dependent, _ := frontend.New(tt.opts).(plugin.Dependent)
				_, err := dependent.Dependencies(t.Context(), round,
					roundReader{storeTree{fstest.MapFS{}, tt.stores}})
				assert.NoError(t, err, "the round places what it can")
				assert.Equal(t, round.Unplaced(), tt.want, "the round reports the expected needs with their reasons")
			})
		}
	})
}

// libStores returns the fake ct.sym beside a Maven store that contains the
// JAR of the library with the given bytes and their SHA-1 record.
func libStores(tb testing.TB, data []byte) map[string]fs.FS {
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

// mavenStore returns a Maven store with the JAR of the library, beside a
// record of a digest and the name of the JAR in the form that Maven
// writes.
func mavenStore(record string) fstest.MapFS {
	return fstest.MapFS{
		mavenJAR:    {Data: []byte(jarBytes)},
		mavenRecord: {Data: []byte(record + "  " + jarName + "\n")},
	}
}

// gradleStore returns a Gradle store with the JAR of the library, with the
// given bytes, in the given directory.
func gradleStore(data, dir string) fstest.MapFS {
	return fstest.MapFS{path.Join(gradleVersion, dir, jarName): {Data: []byte(data)}}
}

// zeroLedJAR returns the first bytes of the form "jar N" whose SHA-1
// digest starts with a zero.
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

// runRound runs a round of a number through a Java frontend with options,
// over an empty workspace and stores, for the given needs, under the
// context of the test.
func runRound(
	tb testing.TB, opts *frontend.Options, stores map[string]fs.FS, number int, needs ...string,
) ([][]plugin.SourceRef, error) {
	tb.Helper()

	round := &plugin.DependencyRound{Number: number}
	for _, need := range needs {
		round.Needs = append(round.Needs, plugin.Need{Path: need})
	}
	dependent, _ := frontend.New(opts).(plugin.Dependent)
	return dependent.Dependencies(tb.Context(), round, roundReader{storeTree{fstest.MapFS{}, stores}})
}

// rounds runs a round that succeeds in the case and returns its units.
func rounds(
	tb testing.TB, opts *frontend.Options, stores map[string]fs.FS, number int, needs ...string,
) [][]plugin.SourceRef {
	tb.Helper()

	units, err := runRound(tb, opts, stores, number, needs...)
	assert.NoError(tb, err, "the round places its needs")
	return units
}

// placed runs a round that succeeds in the case and returns the member
// paths of each unit.
func placed(tb testing.TB, opts *frontend.Options, stores map[string]fs.FS, number int, needs ...string) [][]string {
	tb.Helper()

	units := rounds(tb, opts, stores, number, needs...)
	out := make([][]string, 0, len(units))
	for _, unit := range units {
		var members []string
		for _, ref := range unit {
			members = append(members, ref.Path)
		}
		out = append(out, members)
	}
	return out
}

// inJDK returns the qualified path of an entry of ct.sym.
func inJDK(p string) string { return plugin.StorePath(frontend.JDKStore, p) }

// inMaven returns the qualified path of a file of the Maven store.
func inMaven(p string) string { return plugin.StorePath(frontend.MavenStore, p) }

// inGradle returns the qualified path of the JAR of the library in a
// digest directory of the Gradle store.
func inGradle(dir string) string {
	return plugin.StorePath(frontend.GradleStore, path.Join(gradleVersion, dir, jarName))
}
