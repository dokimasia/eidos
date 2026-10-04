// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The variables Stores reads, and the paths it resolves under them.
const (
	envJavaHome   = "JAVA_HOME"
	envHome       = "HOME"
	envGradleHome = "GRADLE_USER_HOME"
	ctSymFile     = "lib/ct.sym"
	m2Dir         = ".m2/repository"
	gradleDir     = "caches/modules-2/files-2.1"
	defaultGradle = ".gradle"
	probeFile     = "probe.txt"
	objectClass   = "Object"
	stringClass   = "String"
)

// fileMode is the mode the store cases write their files with.
const fileMode = 0o644

// dirMode is the mode the store cases create their directories with.
const dirMode = 0o755

// The homes the allocation cases name. Stores reads no directory under
// either, so neither has to exist.
const (
	userHome   = "/home"
	gradleHome = "/gradle"
)

// The allocations of the stores.
const (
	// storesAllocs is the stores over a ct.sym of one entry: the file's
	// read, its reader and its ZIP index, the four paths Stores joins, and
	// the map with its two directory roots.
	storesAllocs = 5 + 1 + 13 + 4 + 2 + 2
	// gradleStoresAllocs is the stores where GRADLE_USER_HOME names
	// Gradle's home, which joins one path fewer.
	gradleStoresAllocs = storesAllocs - 1
)

// Stores resolves each store's root from the environment, so the
// variable each root comes from, the defaults and the refusals are
// pinned over temporary trees, and the JDK's own ct.sym is loaded where
// JAVA_HOME names one.
func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("Stores", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming JAVA_HOME for an unset JAVA_HOME", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(env(nil))
			assert.True(t, strings.Contains(err.Error(), envJavaHome), "the variable to set")
		})

		t.Run("returns an error naming HOME for an unset HOME", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t)}))
			assert.True(t, strings.Contains(err.Error(), envHome), "the variable to set")
		})

		t.Run("returns an error for a JDK without ct.sym", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(env(map[string]string{envJavaHome: t.TempDir(), envHome: t.TempDir()}))
			assert.ErrorIs(t, err, fs.ErrNotExist, "lib/ct.sym is missing")
		})

		t.Run("returns an error for a ct.sym that is no ZIP file", func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			writeFile(t, home, ctSymFile, []byte("junk"))
			_, err := frontend.Stores(env(map[string]string{envJavaHome: home, envHome: t.TempDir()}))
			assert.HasError(t, err, "the file does not open")
		})

		t.Run("opens ct.sym as the JDK store", func(t *testing.T) {
			t.Parallel()

			stores, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t), envHome: t.TempDir()}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.JDKStore], lang9To25)
			assert.NoError(t, err, "the entry reads")
		})

		t.Run("roots the Maven store at .m2/repository under HOME", func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			writeFile(t, home, filepath.Join(m2Dir, probeFile), nil)
			stores, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t), envHome: home}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.MavenStore], probeFile)
			assert.NoError(t, err, "the repository's file reads")
		})

		t.Run("roots the Gradle store under GRADLE_USER_HOME", func(t *testing.T) {
			t.Parallel()

			gradle := t.TempDir()
			writeFile(t, gradle, filepath.Join(gradleDir, probeFile), nil)
			stores, err := frontend.Stores(env(map[string]string{
				envJavaHome: jdkHome(t), envHome: t.TempDir(), envGradleHome: gradle,
			}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.GradleStore], probeFile)
			assert.NoError(t, err, "the module cache's file reads")
		})

		t.Run("roots the Gradle store under .gradle in HOME by default", func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			writeFile(t, home, filepath.Join(defaultGradle, gradleDir, probeFile), nil)
			stores, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t), envHome: home}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.GradleStore], probeFile)
			assert.NoError(t, err, "the module cache's file reads")
		})

		t.Run("loads java.lang from the ct.sym JAVA_HOME names", func(t *testing.T) {
			t.Parallel()

			decls := machineLang(t)
			named[*node.Struct](t, decls, objectClass)
			named[*node.Struct](t, decls, stringClass)
		})

		t.Run("lowers Object from the ct.sym JAVA_HOME names without a superclass", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, named[*node.Struct](t, machineLang(t), objectClass).Extends, "Object extends nothing")
		})
	})
}

// The stores allocate ct.sym's read and index, the paths and the map.
// The ordinary run, which runs no benchmark, checks those ceilings here.
func TestStoreAllocs(t *testing.T) {
	checkAllocs(t, storeCalls(t))
}

// BenchmarkStore measures the resolution a load makes once, before it
// reads any dependency unit.
func BenchmarkStore(b *testing.B) {
	benchCalls(b, storeCalls(b))
}

// storeCalls returns a call of Stores over a ct.sym of one entry, with
// Gradle's home by default and named.
func storeCalls(tb testing.TB) []allocCall {
	tb.Helper()

	jdk := jdkHome(tb)
	byDefault := env(map[string]string{envJavaHome: jdk, envHome: userHome})
	named := env(map[string]string{envJavaHome: jdk, envHome: userHome, envGradleHome: gradleHome})
	var (
		stores map[string]fs.FS
		err    error
	)
	check := func(tb assert.TB) {
		assert.NoError(tb, err, "Stores resolves every root")
		assert.Length(tb, stores, 3, "Stores returns ct.sym, the Maven repository and Gradle's cache")
	}
	return []allocCall{
		{
			name: "Stores", allocs: storesAllocs,
			call:  func() { stores, err = frontend.Stores(byDefault) },
			check: check,
		},
		{
			name: "Stores/a Gradle home", allocs: gradleStoresAllocs,
			call:  func() { stores, err = frontend.Stores(named) },
			check: check,
		},
	}
}

// machineLang returns the declarations of java.lang that the first round
// places in the ct.sym JAVA_HOME names, parsed at signature depth. It
// skips the case without JAVA_HOME, and fails it there in CI.
func machineLang(tb testing.TB) node.Symbols {
	tb.Helper()

	if os.Getenv(envJavaHome) == "" && toolchain.RequiredInCI() {
		tb.Fatal("JAVA_HOME names no JDK, which CI requires")
	}
	if os.Getenv(envJavaHome) == "" {
		tb.Skip("JAVA_HOME names no JDK, which CI provides")
	}
	stores, err := frontend.Stores(os.Getenv)
	assert.NoError(tb, err, "the machine's stores resolve")
	units := rounds(tb, nil, stores, 1)
	assert.Length(tb, units, 1, "java/lang alone")
	f := frontend.New(nil)
	u := plugin.NewSourceUnit(units[0], storeTree{fstest.MapFS{}, stores}, plugin.DepthSignatures, f.Syntax(), brand,
		diag.NewSink(), f.Name())
	assert.NoError(tb, f.Parse(context.Background(), u), "java/lang parses")
	return packageDecls(tb, u.Graph(), langPackage)
}

// env returns a getenv over a map of variables, empty for any other.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// jdkHome returns a temporary JDK home whose lib/ct.sym is a ZIP file of
// the fake ct.sym's java.lang entry.
func jdkHome(tb testing.TB) string {
	tb.Helper()

	home := tb.TempDir()
	writeFile(tb, home, ctSymFile, jarOf(tb, "", map[string][]byte{lang9To25: nil}))
	return home
}

// writeFile writes a file under a directory, creating its parents.
func writeFile(tb assert.TB, dir, rel string, data []byte) {
	tb.Helper()

	p := filepath.Join(dir, filepath.FromSlash(rel))
	assert.NoError(tb, os.MkdirAll(filepath.Dir(p), dirMode), "the parents are created")
	assert.NoError(tb, os.WriteFile(p, data, fileMode), "the file is written")
}
