// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// The parts of a library in the classpath option: the number of parts,
// which are the group, the artifact and the version, the separator of a
// group as a path, the separator in the name of a JAR, the suffix of the
// SHA-1 record, and the digit that Gradle leaves off the front of a digest.
const (
	coordinateParts = 3
	groupDot        = "."
	versionDash     = "-"
	sha1Suffix      = ".sha1"
	leadingZero     = "0"
)

// The releases that ct.sym names by one character each: the first and the
// last release, the release that the letters count from, the first and the
// last digit, and the first and the last letter.
const (
	oldestRelease = 8
	letterRelease = 10
	lastRelease   = 35
	firstDigit    = '0'
	lastDigit     = '9'
	firstLetter   = 'A'
	lastLetter    = 'Z'
)

// coordinate is one library of the classpath option.
type coordinate struct {
	group    string
	artifact string
	version  string
}

// parseCoordinate parses the group:artifact:version of a library. It
// returns an error for a specification without three parts that are not
// empty.
func parseCoordinate(spec string) (coordinate, error) {
	parts := strings.Split(spec, coordinateSeparator)
	if len(parts) != coordinateParts || slices.Contains(parts, "") {
		return coordinate{}, fmt.Errorf("frontend: the classpath library %q is not group:artifact:version", spec)
	}
	return coordinate{group: parts[0], artifact: parts[1], version: parts[2]}, nil
}

// jar returns the file name of the JAR of the library.
func (c coordinate) jar() string { return c.artifact + versionDash + c.version + jarExt }

// mavenJAR returns the qualified path of the JAR of the library in the
// Maven local repository, whose path has one directory per name of the
// group.
func (c coordinate) mavenJAR() string {
	group := strings.ReplaceAll(c.group, groupDot, binarySlash)
	return plugin.StorePath(MavenStore, path.Join(group, c.artifact, c.version, c.jar()))
}

// gradleDir returns the qualified path of the directory in the module
// cache of Gradle that contains the files of the library, one directory
// per digest.
func (c coordinate) gradleDir() string {
	return plugin.StorePath(GradleStore, path.Join(c.group, c.artifact, c.version))
}

// jdkIndex maps each package of one release in ct.sym to the qualified
// paths of its class files, in path order, which is the order of the walk.
type jdkIndex map[string][]string

// unitOf returns the package of a need and its unit. The package is the
// need itself where ct.sym has that package, and otherwise the package of
// the class that the need names, as [packageIn] finds it. unitOf returns
// no unit for a need whose package ct.sym does not have.
func (x jdkIndex) unitOf(need string) (string, []plugin.SourceRef) {
	p := packageIn(x, need)
	if p == "" {
		return "", nil
	}
	unit := make([]plugin.SourceRef, 0, len(x[p]))
	for _, f := range x[p] {
		unit = append(unit, plugin.SourceRef{Path: f})
	}
	return p, unit
}

// packageIn returns the package of a need among packages keyed by path.
// The package is the need itself where the map has it. Otherwise the need
// is a class, as the need of a static import or of a single-type import of
// a member class is, and packageIn drops its last names one at a time. It
// returns the empty string where the map has no such package.
func packageIn[V any](packages map[string]V, need string) string {
	for p := need; p != "."; p = path.Dir(p) {
		if _, met := packages[p]; met {
			return p
		}
	}
	return ""
}

// dependencies is the dependency round of the Java frontend. The first
// round returns one unit for each library of the classpath option, which
// is its JAR, and the unit of java/lang. Every round returns the JDK
// packages of its needs from the entries of ct.sym for the release of the
// options, and for the newest release where the options name none. A
// store that the load does not provide turns its source off.
//
// Error modes: dependencies returns an error for a library that the option
// writes in a wrong form, for a library that neither the Maven nor the
// Gradle store has, for a JAR whose SHA-1 digest differs from its record,
// and for a release for which ct.sym lists no entry.
//
// The round reports a need as placed nowhere where neither ct.sym nor a
// JAR of the classpath has its package. The first round reads the packages
// of the JARs that it returns. The needs of a later round are packages
// that no loaded JAR declares.
func (f *javaFrontend) dependencies(
	ctx context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	var out [][]plugin.SourceRef
	needs := make([]string, 0, len(round.Needs)+1)
	classpath := map[string]bool{}
	if round.Number == 1 {
		for _, spec := range f.opts.Classpath {
			unit, data, err := library(r, spec)
			if err != nil {
				return nil, err
			}
			out = append(out, unit)
			maps.Copy(classpath, jarPackages(data, f.opts.Release))
		}
		needs = append(needs, javaLang)
	}
	for _, n := range round.Needs {
		needs = append(needs, n.Path)
	}
	jdk, provided, err := readJDK(ctx, r, f.opts.Release)
	if err != nil {
		return nil, err
	}
	placed := map[string]bool{}
	for _, need := range needs {
		if pkg, unit := jdk.unitOf(need); unit != nil && !placed[pkg] {
			placed[pkg] = true
			out = append(out, unit)
		}
	}
	reason := "neither the " + JDKStore + " store nor a classpath library has its package"
	if !provided {
		reason = "the load provides no " + JDKStore + " store, and no classpath library has its package"
	}
	for _, n := range round.Needs {
		if packageIn(jdk, n.Path) == "" && packageIn(classpath, n.Path) == "" {
			round.Unplace(n.Path, reason)
		}
	}
	return out, nil
}

// library returns the unit of one library of the classpath and the bytes
// of its JAR. The JAR comes from the Maven store, and the SHA-1 record of
// the JAR is the shared input of the unit. Where the Maven store does not
// have the JAR, it comes from the Gradle store.
func library(r plugin.StoreReader, spec string) ([]plugin.SourceRef, []byte, error) {
	c, err := parseCoordinate(spec)
	if err != nil {
		return nil, nil, err
	}
	unit, data, found, err := fromMaven(r, c)
	if err != nil || found {
		return unit, data, err
	}
	unit, data, found, err = fromGradle(r, c)
	if err != nil || found {
		return unit, data, err
	}
	return nil, nil, fmt.Errorf("frontend: neither the %s store nor the %s store has the classpath library %s",
		MavenStore, GradleStore, spec)
}

// fromMaven returns the unit of the JAR of a library in the Maven store
// and the bytes of the JAR, and reports whether the store has the JAR. The
// SHA-1 record beside the JAR must contain the digest of the JAR as its
// first word, compared without regard to case.
func fromMaven(r plugin.StoreReader, c coordinate) ([]plugin.SourceRef, []byte, bool, error) {
	jar := c.mavenJAR()
	data, err := r.Read(jar)
	if plugin.Absent(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	record := jar + sha1Suffix
	want, err := r.Read(record)
	if err != nil {
		return nil, nil, false, fmt.Errorf("frontend: %s has no SHA-1 record to check it against: %w", jar, err)
	}
	fields := strings.Fields(string(want))
	if len(fields) == 0 || !strings.EqualFold(fields[0], digest(data)) {
		return nil, nil, false, fmt.Errorf("frontend: the SHA-1 digest of %s is %s, and %s records %q",
			jar, digest(data), record, strings.TrimSpace(string(want)))
	}
	return []plugin.SourceRef{{Path: jar, Shared: []string{record}}}, data, true, nil
}

// fromGradle returns the unit of the JAR of a library in the Gradle store
// and the bytes of the JAR, and reports whether the store has the JAR. The
// name of the directory of the JAR is its SHA-1 digest, which Gradle writes
// without leading zeros, so the two compare without them.
func fromGradle(r plugin.StoreReader, c coordinate) ([]plugin.SourceRef, []byte, bool, error) {
	dir := c.gradleDir()
	entries, err := r.ReadDir(dir)
	if plugin.Absent(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		jar := dir + binarySlash + e.Name() + binarySlash + c.jar()
		data, err := r.Read(jar)
		if plugin.Absent(err) {
			continue
		}
		if err != nil {
			return nil, nil, false, err
		}
		if strings.TrimLeft(e.Name(), leadingZero) != strings.TrimLeft(digest(data), leadingZero) {
			return nil, nil, false, fmt.Errorf(
				"frontend: the SHA-1 digest of %s is %s, and the name of its directory is %s",
				jar, digest(data), e.Name())
		}
		return []plugin.SourceRef{{Path: jar}}, data, true, nil
	}
	return nil, nil, false, nil
}

// readJDK indexes the packages that ct.sym has for a release. It walks the
// directories whose names include the character of the release. It also
// reports whether the load provides the JDK store, and returns an empty
// index for a load without it. Release 0 is the newest release that
// ct.sym lists.
//
// Error modes: readJDK returns an error where ct.sym lists no entry for
// the release, and the error of a listing that fails.
func readJDK(ctx context.Context, r plugin.StoreReader, release int) (jdkIndex, bool, error) {
	root := plugin.StorePath(JDKStore, "")
	tops, err := r.ReadDir(root)
	if errors.Is(err, plugin.ErrStoreAbsent) {
		return jdkIndex{}, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	mark, err := releaseMark(release, tops)
	if err != nil {
		return nil, true, err
	}
	x := jdkIndex{}
	for _, top := range tops {
		if !top.IsDir() || !strings.ContainsRune(top.Name(), mark) {
			continue
		}
		modules, err := r.ReadDir(root + top.Name())
		if err != nil {
			return nil, true, err
		}
		for _, m := range modules {
			if m.IsDir() {
				if err := x.walk(ctx, r, root+top.Name()+binarySlash+m.Name(), ""); err != nil {
					return nil, true, err
				}
			}
		}
	}
	return x, true, nil
}

// walk adds the class files of a module directory to their packages, below
// a package path. The declaration of the module itself, module-info.sig,
// is indexed under the empty package path, which no need names.
func (x jdkIndex) walk(ctx context.Context, r plugin.StoreReader, dir, pkg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := r.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir():
			if err := x.walk(ctx, r, dir+binarySlash+name, path.Join(pkg, name)); err != nil {
				return err
			}
		case path.Ext(name) == sigExt:
			x[pkg] = append(x[pkg], dir+binarySlash+name)
		}
	}
	return nil
}

// releaseMark returns the character by which ct.sym names a release: 8 and
// 9 for themselves, and a letter from A for 10 on. Release 0 is the newest
// character in the names of the top directories.
//
// Error modes: releaseMark returns an error for a release that ct.sym
// cannot name, and for a release that no name of a top directory
// contains.
func releaseMark(release int, tops []fs.DirEntry) (rune, error) {
	newest := rune(0)
	for _, top := range tops {
		for _, c := range top.Name() {
			if top.IsDir() && rank(c) > rank(newest) {
				newest = c
			}
		}
	}
	if release == 0 {
		if newest == 0 {
			return 0, errors.New("frontend: ct.sym lists no release")
		}
		return newest, nil
	}
	if release < oldestRelease || release > lastRelease {
		return 0, fmt.Errorf("frontend: ct.sym names no release %d", release)
	}
	mark := firstDigit + rune(release)
	if release >= letterRelease {
		mark = firstLetter + rune(release-letterRelease)
	}
	if rank(mark) > rank(newest) {
		return 0, fmt.Errorf("frontend: ct.sym lists no release %d", release)
	}
	return mark, nil
}

// rank orders the characters of releases, the digits before the letters,
// as ct.sym orders releases.
func rank(c rune) int {
	switch {
	case c >= firstDigit && c <= lastDigit:
		return int(c - firstDigit)
	case c >= firstLetter && c <= lastLetter:
		return letterRelease + int(c-firstLetter)
	default:
		return -1
	}
}

// digest returns the SHA-1 digest of a JAR in lowercase hexadecimal, the
// digest that Maven and Gradle record. The digest detects a corrupt
// download and is not a security check.
func digest(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}
