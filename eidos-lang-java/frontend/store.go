// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// The stores the Java frontend reads dependency units from, under the
// names a load lists them by.
const (
	// JDKStore is the JDK's ct.sym: the class files of each release's
	// documented API, at "<release characters>/<module>/<package
	// path>/<class>.sig", where each character of the first directory's
	// name is a release the entries below it are part of.
	JDKStore = "jdk"

	// MavenStore is the Maven local repository: a library's JAR at
	// "<group path>/<artifact>/<version>/<artifact>-<version>.jar",
	// beside its SHA-1 record, which adds ".sha1".
	MavenStore = "m2"

	// GradleStore is Gradle's module cache: a library's JAR at
	// "<group>/<artifact>/<version>/<SHA-1>/<artifact>-<version>.jar",
	// the directory named by the JAR's SHA-1 digest.
	GradleStore = "gradle"
)

// The variables Stores reads: the JDK's home, the user's home, and
// Gradle's home.
const (
	envJavaHome   = "JAVA_HOME"
	envHome       = "HOME"
	envGradleHome = "GRADLE_USER_HOME"
)

// The slash paths under the roots Stores resolves: ct.sym under the
// JDK's home, the local repository under the user's home, Gradle's home
// under the user's home, and the module cache under Gradle's home.
const (
	ctSymPath     = "lib/ct.sym"
	m2Repository  = ".m2/repository"
	gradleDefault = ".gradle"
	gradleCache   = "caches/modules-2/files-2.1"
)

// Stores returns ct.sym, the Maven local repository and Gradle's module
// cache as the stores the load passes to the Java frontend, under
// [JDKStore], [MavenStore] and [GradleStore]. It reads the environment
// through getenv, which is os.Getenv outside a test, and runs no tool.
//
// ct.sym is lib/ct.sym under JAVA_HOME, read into memory whole, because a
// store has no Close that would release an open file. The local
// repository is .m2/repository under HOME. The module cache is
// caches/modules-2/files-2.1 under GRADLE_USER_HOME, which defaults to
// .gradle under HOME.
//
// Stores returns an error naming the variable to set for a root it
// cannot resolve, and an error for a ct.sym that does not read or open
// as a ZIP file.
//
// # Allocation contract
//
// Stores allocates ct.sym's contents and the ZIP index over them, which
// grows with the entries, the four paths it joins, and the map of
// stores with its two directory roots: 27 allocations for a ct.sym of
// one entry, and one fewer where GRADLE_USER_HOME is set.
func Stores(getenv func(string) string) (map[string]fs.FS, error) {
	javaHome := getenv(envJavaHome)
	if javaHome == "" {
		return nil, errors.New("frontend: set JAVA_HOME, the JDK whose lib/ct.sym the Java frontend reads")
	}
	home := getenv(envHome)
	if home == "" {
		return nil, errors.New("frontend: set HOME, the directory the Maven repository and the Gradle home are under")
	}
	ctSym := filepath.Join(javaHome, filepath.FromSlash(ctSymPath))
	data, err := os.ReadFile(ctSym)
	if err != nil {
		return nil, fmt.Errorf("frontend: %w", err)
	}
	jdk, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("frontend: %s: %w", ctSym, err)
	}
	gradleHome := getenv(envGradleHome)
	if gradleHome == "" {
		gradleHome = filepath.Join(home, gradleDefault)
	}
	return map[string]fs.FS{
		JDKStore:    jdk,
		MavenStore:  os.DirFS(filepath.Join(home, filepath.FromSlash(m2Repository))),
		GradleStore: os.DirFS(filepath.Join(gradleHome, filepath.FromSlash(gradleCache))),
	}, nil
}
