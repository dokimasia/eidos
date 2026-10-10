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

// The spellings of a library the classpath option names: its count of
// parts, which are its group, artifact and version, the separators of a
// group as a path and of a JAR's name, the suffix of its SHA-1 record,
// and the digit Gradle leaves off the front of a digest.
const (
	coordinateParts = 3
	groupDot        = "."
	versionDash     = "-"
	sha1Suffix      = ".sha1"
	leadingZero     = "0"
)

// The first and the last release ct.sym names by one character each, the
// release the letters count from, and the first digit and the first and
// last letter of the names.
const (
	oldestRelease = 8
	letterRelease = 10
	lastRelease   = 35
	firstDigit    = '0'
	lastDigit     = '9'
	firstLetter   = 'A'
	lastLetter    = 'Z'
)

// coordinate is one library the classpath option names.
type coordinate struct {
	group    string
	artifact string
	version  string
}

// parseCoordinate parses a library's group:artifact:version, and
// returns an error for one without three parts that are not empty.
func parseCoordinate(spec string) (coordinate, error) {
	parts := strings.Split(spec, coordinateSeparator)
	if len(parts) != coordinateParts || slices.Contains(parts, "") {
		return coordinate{}, fmt.Errorf("frontend: the classpath library %q is no group:artifact:version", spec)
	}
	return coordinate{group: parts[0], artifact: parts[1], version: parts[2]}, nil
}

// jar returns the file name of the library's JAR.
func (c coordinate) jar() string { return c.artifact + versionDash + c.version + jarExt }

// mavenJAR returns the qualified path of the library's JAR in the Maven
// local repository, whose path spells the group with a directory per
// name.
func (c coordinate) mavenJAR() string {
	group := strings.ReplaceAll(c.group, groupDot, binarySlash)
	return plugin.StorePath(MavenStore, path.Join(group, c.artifact, c.version, c.jar()))
}

// gradleDir returns the qualified path of the directory Gradle's module
// cache keeps the library's files in, one directory per digest.
func (c coordinate) gradleDir() string {
	return plugin.StorePath(GradleStore, path.Join(c.group, c.artifact, c.version))
}

// jdkIndex is one release's packages in ct.sym: each package's class
// files, by their qualified paths in path order, the order the walk
// visits them in.
type jdkIndex map[string][]string

// unitOf returns the package a need names and its unit: the need itself
// where ct.sym has that package, and otherwise the package of the class
// it names, [packageIn]. It returns no unit for a need ct.sym has no
// package for.
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

// packageIn returns the package a need names among packages keyed by
// path: the need itself where the map has it, and otherwise the package
// of the class it names, as a static import's need and a single-type
// import of a member class name a class, its last names left out one at
// a time. It returns "" where the map has none.
func packageIn[V any](packages map[string]V, need string) string {
	for p := need; p != "."; p = path.Dir(p) {
		if _, met := packages[p]; met {
			return p
		}
	}
	return ""
}

// dependencies is the Java frontend's dependency round. The first round
// returns one unit per library the classpath option names, its JAR, and
// java/lang. Every round returns the JDK packages its needs name, from
// ct.sym's entries for the release the options name, the newest where
// they name none. A store the load does not provide turns its source
// off. A library malformed in the option, one neither the Maven nor the
// Gradle store has, a JAR whose SHA-1 digest is not its record's, and a
// release ct.sym lists no entry for are errors.
//
// The round reports a need placed nowhere where neither ct.sym nor a
// classpath JAR has its package. The first round reads the packages
// from the JARs it returns, and a later round's needs are packages no
// loaded JAR declares.
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

// library returns the unit of one classpath library and its JAR's
// bytes: its JAR from the Maven store, whose SHA-1 record is the unit's
// shared input, and from the Gradle store where the Maven store lacks
// it.
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

// fromMaven returns the unit of a library's JAR in the Maven store and
// the JAR's bytes, and reports whether the store has the JAR. The SHA-1
// record beside the JAR must state the JAR's digest, its first word
// compared without case.
func fromMaven(r plugin.StoreReader, c coordinate) ([]plugin.SourceRef, []byte, bool, error) {
	jar := c.mavenJAR()
	data, err := r.Read(jar)
	if absent(err) {
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

// fromGradle returns the unit of a library's JAR in the Gradle store and
// the JAR's bytes, and reports whether the store has the JAR. The JAR's
// directory is named by its SHA-1 digest, which Gradle writes without
// leading zeros, so the two compare without them.
func fromGradle(r plugin.StoreReader, c coordinate) ([]plugin.SourceRef, []byte, bool, error) {
	dir := c.gradleDir()
	entries, err := r.ReadDir(dir)
	if absent(err) {
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
		if absent(err) {
			continue
		}
		if err != nil {
			return nil, nil, false, err
		}
		if strings.TrimLeft(e.Name(), leadingZero) != strings.TrimLeft(digest(data), leadingZero) {
			return nil, nil, false, fmt.Errorf("frontend: the SHA-1 digest of %s is %s, and its directory names %s",
				jar, digest(data), e.Name())
		}
		return []plugin.SourceRef{{Path: jar}}, data, true, nil
	}
	return nil, nil, false, nil
}

// readJDK indexes the packages ct.sym has for a release, by a walk of
// the directories whose names include the release's character, and
// reports whether the load provides the JDK store, returning an empty
// index for a load without it. Release 0 is the newest release ct.sym
// lists. ct.sym listing no entry for the release is an error.
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

// walk adds a module directory's class files to their packages, below a
// package path. A module's own declaration, module-info.sig, is indexed
// under the empty package path, which no need names.
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

// releaseMark returns the character ct.sym names a release by: 8 and 9
// for themselves, and a letter from A for 10 on. Release 0 is the newest
// character the top directories' names have. A release ct.sym cannot
// name, and one no top directory's name has, is an error.
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

// rank orders release characters: the digits before the letters, as
// ct.sym orders releases.
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
// digest Maven and Gradle record, which detects a corrupt download and
// is no security check.
func digest(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

// absent reports whether a read or a listing found nothing to read: the
// file does not exist, or the load provides no store for it.
func absent(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, plugin.ErrStoreAbsent)
}
