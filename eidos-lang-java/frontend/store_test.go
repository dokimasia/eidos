// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// homeVar is the variable of the home directory of the user, which Stores
// reads.
var homeVar = plugin.UserHomeVar()

// The variables that Stores reads, and the paths that it resolves under
// them.
const (
	envJavaHome   = "JAVA_HOME"
	envGradleHome = "GRADLE_USER_HOME"
	ctSymFile     = "lib/ct.sym"
	m2Dir         = ".m2/repository"
	gradleDir     = "caches/modules-2/files-2.1"
	defaultGradle = ".gradle"
	probeFile     = "probe.txt"
	objectClass   = "Object"
	stringClass   = "String"
)

// fileMode is the mode of the files that the store cases write.
const fileMode = 0o644

// dirMode is the mode of the directories that the store cases create.
const dirMode = 0o755

// The homes of the allocation cases. Stores reads no directory under
// either, so neither has to exist.
const (
	userHome   = "/home"
	gradleHome = "/gradle"
)

// The allocations of the stores.
const (
	// storesAllocs is the cost of the stores over a ct.sym of one entry:
	// the read of the file, its reader and its ZIP index, the four paths
	// that Stores joins, and the map with its two directory roots.
	storesAllocs = 5 + 1 + 13 + 4 + 2 + 2
	// gradleStoresAllocs is the cost of the stores where GRADLE_USER_HOME
	// is set, because Stores joins one path fewer.
	gradleStoresAllocs = storesAllocs - 1
)

// Stores resolves the root of each store from the environment. The cases
// pin the variable of each root, the defaults and the errors over
// temporary trees, and load the ct.sym of the JDK of the machine where
// JAVA_HOME is set.
func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("Stores", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error that contains JAVA_HOME where JAVA_HOME is unset", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(env(nil))
			assert.HasError(t, err, "an unset JAVA_HOME resolves no store")
			assert.Contains(t, err.Error(), envJavaHome, "the variable to set")
		})

		t.Run("returns an error that contains the home variable where the home is unset", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t)}))
			assert.HasError(t, err, "an unset home resolves no store")
			assert.Contains(t, err.Error(), homeVar, "the variable to set")
		})

		t.Run("returns an error for a JDK without ct.sym", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(env(map[string]string{envJavaHome: t.TempDir(), homeVar: t.TempDir()}))
			assert.ErrorIs(t, err, fs.ErrNotExist, "lib/ct.sym is missing")
		})

		t.Run("returns an error for a ct.sym that is not a ZIP file", func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			writeFile(t, home, ctSymFile, []byte("junk"))
			_, err := frontend.Stores(env(map[string]string{envJavaHome: home, homeVar: t.TempDir()}))
			assert.HasError(t, err, "the file does not open")
		})

		t.Run("opens ct.sym as the JDK store", func(t *testing.T) {
			t.Parallel()

			stores, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t), homeVar: t.TempDir()}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.JDKStore], lang9To25)
			assert.NoError(t, err, "the entry reads")
		})

		t.Run("roots the Maven store at .m2/repository under the home directory", func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			writeFile(t, home, filepath.Join(m2Dir, probeFile), nil)
			stores, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t), homeVar: home}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.MavenStore], probeFile)
			assert.NoError(t, err, "the repository's file reads")
		})

		t.Run("roots the Gradle store under GRADLE_USER_HOME", func(t *testing.T) {
			t.Parallel()

			gradle := t.TempDir()
			writeFile(t, gradle, filepath.Join(gradleDir, probeFile), nil)
			stores, err := frontend.Stores(env(map[string]string{
				envJavaHome: jdkHome(t), homeVar: t.TempDir(), envGradleHome: gradle,
			}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.GradleStore], probeFile)
			assert.NoError(t, err, "the module cache's file reads")
		})

		t.Run("roots the Gradle store under .gradle in the home directory by default", func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			writeFile(t, home, filepath.Join(defaultGradle, gradleDir, probeFile), nil)
			stores, err := frontend.Stores(env(map[string]string{envJavaHome: jdkHome(t), homeVar: home}))
			assert.NoError(t, err, "the stores resolve")
			_, err = fs.ReadFile(stores[frontend.GradleStore], probeFile)
			assert.NoError(t, err, "the module cache's file reads")
		})

		t.Run("loads java.lang from the ct.sym under JAVA_HOME", func(t *testing.T) {
			t.Parallel()

			decls := machineLang(t)
			named[*node.Struct](t, decls, objectClass)
			named[*node.Struct](t, decls, stringClass)
		})

		t.Run("lowers Object from the ct.sym under JAVA_HOME without a superclass", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, named[*node.Struct](t, machineLang(t), objectClass).Extends, "Object extends nothing")
		})
	})
}

// The stores allocate the read and the index of ct.sym, the paths and the
// map. The ordinary run, which runs no benchmark, checks those ceilings
// here. Darwin and Windows allocate more in their file system calls and
// path functions, so the test runs on Linux alone.
func TestStoreAllocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the ceilings count the allocations of Linux's file system calls")
	}
	checkAllocs(t, storeCalls(t))
}

// BenchmarkStore measures the resolution that a load makes once, before it
// reads any dependency unit.
func BenchmarkStore(b *testing.B) {
	benchCalls(b, storeCalls(b))
}

// storeCalls returns the calls of Stores over a ct.sym of one entry, with
// the default home of Gradle and with GRADLE_USER_HOME set.
func storeCalls(tb testing.TB) []allocCall {
	tb.Helper()

	jdk := jdkHome(tb)
	byDefault := env(map[string]string{envJavaHome: jdk, homeVar: userHome})
	named := env(map[string]string{envJavaHome: jdk, homeVar: userHome, envGradleHome: gradleHome})
	var (
		stores map[string]fs.FS
		err    error
	)
	check := func(tb testing.TB) {
		tb.Helper()

		assert.NoError(tb, err, "Stores resolves every root")
		assert.Length(tb, stores, 3, "Stores returns ct.sym, the Maven repository and Gradle's cache")
	}
	return []allocCall{
		{
			name: "Stores", caseName: "the default homes", allocs: storesAllocs,
			call:  func() { stores, err = frontend.Stores(byDefault) },
			check: check,
		},
		{
			name: "Stores", caseName: "a Gradle home", allocs: gradleStoresAllocs,
			call:  func() { stores, err = frontend.Stores(named) },
			check: check,
		},
	}
}

// machineLang returns the declarations of java.lang that the first round
// places in the ct.sym under JAVA_HOME, parsed at signature depth. It
// skips the case where JAVA_HOME is unset, and fails it in CI.
func machineLang(tb testing.TB) node.Symbols {
	tb.Helper()

	assert.False(
		tb,
		os.Getenv(envJavaHome) == "" && toolchain.RequiredInCI(),
		"JAVA_HOME names a JDK, which CI requires",
	)
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
	assert.NoError(tb, f.Parse(tb.Context(), u), "java/lang parses")
	return packageDecls(tb, u.Graph(), langPackage)
}

// env returns a getenv over a map of variables, which returns the empty
// string for any other variable.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// jdkHome returns a temporary JDK home whose lib/ct.sym is a ZIP file with
// the java.lang entry of the fake ct.sym.
func jdkHome(tb testing.TB) string {
	tb.Helper()

	home := tb.TempDir()
	writeFile(tb, home, ctSymFile, jarOf(tb, "", map[string][]byte{lang9To25: nil}))
	return home
}

// writeFile writes a file under a directory, creating its parents.
func writeFile(tb testing.TB, dir, rel string, data []byte) {
	tb.Helper()

	p := filepath.Join(dir, filepath.FromSlash(rel))
	assert.NoError(tb, os.MkdirAll(filepath.Dir(p), dirMode), "the parents are created")
	assert.NoError(tb, os.WriteFile(p, data, fileMode), "the file is written")
}
