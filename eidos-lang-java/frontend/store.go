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

	"go.dokimi.dev/eidos/sdk/plugin"
)

// The stores from which the Java frontend reads dependency units, under
// the names by which a load lists them.
const (
	// JDKStore is the ct.sym of the JDK: the class files of the documented
	// API of each release, at "<release characters>/<module>/<package
	// path>/<class>.sig". Each character of the name of the first
	// directory is a release that contains the entries below it.
	JDKStore = "jdk"

	// MavenStore is the Maven local repository: the JAR of a library at
	// "<group path>/<artifact>/<version>/<artifact>-<version>.jar", beside
	// its SHA-1 record, whose name adds ".sha1".
	MavenStore = "m2"

	// GradleStore is the module cache of Gradle: the JAR of a library at
	// "<group>/<artifact>/<version>/<SHA-1>/<artifact>-<version>.jar". The
	// name of the directory is the SHA-1 digest of the JAR.
	GradleStore = "gradle"
)

// The variables from which Stores reads the home of the JDK and the home
// of Gradle. Stores reads the home of the user from the variable that
// [plugin.UserHomeVar] names.
const (
	envJavaHome   = "JAVA_HOME"
	envGradleHome = "GRADLE_USER_HOME"
)

// The slash paths under the roots that Stores resolves: ct.sym under the
// home of the JDK, the local repository under the home of the user, the
// home of Gradle under the home of the user, and the module cache under
// the home of Gradle.
const (
	ctSymPath     = "lib/ct.sym"
	m2Repository  = ".m2/repository"
	gradleDefault = ".gradle"
	gradleCache   = "caches/modules-2/files-2.1"
)

// Stores returns ct.sym, the Maven local repository and the module cache
// of Gradle as the stores that the load passes to the Java frontend, under
// [JDKStore], [MavenStore] and [GradleStore]. It reads the environment
// through getenv, which is os.Getenv outside a test, and runs no tool.
//
// ct.sym is lib/ct.sym under JAVA_HOME. Stores reads it into memory whole,
// because a store has no Close that would release an open file. The local
// repository is .m2/repository under the home directory of the user. The
// module cache is caches/modules-2/files-2.1 under GRADLE_USER_HOME, which
// defaults to .gradle under the home directory. Stores reads the home
// directory from the variable that [plugin.UserHomeVar] names.
//
// Error modes: Stores returns an error that names the variable to set for
// a root that it cannot resolve. It returns an error for a ct.sym that
// does not read or does not open as a ZIP file.
//
// # Allocation contract
//
// Stores allocates the contents of ct.sym and the ZIP index over them,
// which grows with the entries, the four paths that it joins, and the map
// of stores with its two directory roots: 27 allocations for a ct.sym of
// one entry, and one fewer where GRADLE_USER_HOME is set.
func Stores(getenv func(string) string) (map[string]fs.FS, error) {
	javaHome := getenv(envJavaHome)
	if javaHome == "" {
		return nil, errors.New("frontend: set JAVA_HOME, the JDK whose lib/ct.sym the Java frontend reads")
	}
	homeName := plugin.UserHomeVar()
	home := getenv(homeName)
	if home == "" {
		return nil, fmt.Errorf("frontend: set %s, the directory of the Maven repository and the Gradle home",
			homeName)
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
