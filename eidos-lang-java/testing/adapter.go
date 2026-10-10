// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The Java toolchain's spellings.
const (
	javacBinary = "javac"
	javaBinary  = "java"
	classExt    = ".class"
	// classesDir is where the project compiles to, and toolDir where
	// the harness writes its own drivers and probe: directories no
	// generated path uses, left out of the project's own sources.
	classesDir = "eidos-classes"
	toolDir    = "eidos-tool"
	probeDir   = "eidos-probe"
	// testSuffix ends the simple name of a test class.
	testSuffix = "Test"
	// nestedMark joins a nested class's name to its host's, and a
	// nested class is never a test by itself.
	nestedMark = "$"
	// dirPerm and filePerm keep the scratch project private to the
	// user running the check.
	dirPerm  = 0o700
	filePerm = 0o600
	// incompatible is javac's raw key for incompatible types, the one
	// refusal that makes a satisfaction probe report false.
	incompatible = "compiler.err.prob.found.req"
	// errorKey opens every raw error key javac reports.
	errorKey = "compiler.err."
)

// The drivers the harness runs through java's source launcher, and
// the lines the test driver prints.
const (
	parseDriver = "EidosParse.java"
	testsDriver = "EidosTests.java"
	probeFile   = "EidosProbe.java"
	passLine    = "eidos: pass "
	failLine    = "eidos: fail "
)

// parseSource reads every file it is handed through javac's own
// parser and nothing after it, so a syntax error is told apart from a
// type error.
const parseSource = `import com.sun.source.util.JavacTask;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Locale;
import javax.tools.Diagnostic;
import javax.tools.DiagnosticCollector;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class EidosParse {
    public static void main(String[] args) throws Exception {
        var compiler = ToolProvider.getSystemJavaCompiler();
        var diagnostics = new DiagnosticCollector<JavaFileObject>();
        try (StandardJavaFileManager files =
                compiler.getStandardFileManager(diagnostics, Locale.ROOT, StandardCharsets.UTF_8)) {
            var units = files.getJavaFileObjectsFromStrings(List.of(args));
            ((JavacTask) compiler.getTask(null, files, diagnostics, List.of(), null, units)).parse();
        }
        int errors = 0;
        for (var d : diagnostics.getDiagnostics()) {
            if (d.getKind() == Diagnostic.Kind.ERROR) {
                errors++;
                System.out.println(d.getSource().getName() + ":" + d.getLineNumber() + ": " + d.getMessage(null));
            }
        }
        System.exit(errors == 0 ? 0 : 1);
    }
}
`

// testsSource runs the main method of every class it is handed and
// prints one line per class: a return passes, and a throw fails.
const testsSource = `import java.lang.reflect.InvocationTargetException;

public final class EidosTests {
    public static void main(String[] args) {
        int failed = 0;
        for (String name : args) {
            try {
                Class.forName(name).getMethod("main", String[].class).invoke(null, (Object) new String[0]);
                System.out.println("` + passLine + `" + name);
            } catch (InvocationTargetException e) {
                failed++;
                System.out.println("` + failLine + `" + name + ": " + e.getCause());
            } catch (ReflectiveOperationException e) {
                failed++;
                System.out.println("` + failLine + `" + name + ": " + e);
            }
        }
        System.exit(failed == 0 ? 0 : 1);
    }
}
`

// runTimeout bounds one toolchain invocation, so a compiler that
// never returns fails the check with an error and leaves the suite
// running.
const runTimeout = 5 * time.Minute

// adapter drives javac and java over generated Java. It has no state,
// so one value serves every check.
type adapter struct{}

// New returns the Java toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [java.Lang], which the kernel's assertions name in
// their failure messages.
func (adapter) Lang() symbol.Lang { return java.Lang }

// Available reports whether javac and java are both on PATH, and
// names the first binary it did not find.
func (adapter) Available() (bool, string) {
	for _, binary := range []string{javacBinary, javaBinary} {
		if _, err := exec.LookPath(binary); err != nil {
			return false, fmt.Sprintf("%s is not on PATH: %v", binary, err)
		}
	}
	return true, ""
}

// Layout writes the generated output as a source tree in a scratch
// directory, every file at its own path. Java needs no build file:
// javac compiles the sources it is handed. A write that fails, a path
// escaping the directory among them, returns an error and removes the
// directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	dir, err := os.MkdirTemp("", "eidos-java-*")
	if err != nil {
		return "", fmt.Errorf("testing: %w", err)
	}
	for path, body := range g.Files {
		if err := write(dir, path, body); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	return dir, nil
}

// Parse reads every Java source's syntax through javac's own parser,
// run as a driver through java's source launcher, and returns an
// error naming the file and line of every syntax error. A project
// with no Java file returns an error, because a parse of nothing
// proves nothing. Java has no parser in Go, so the parse needs the
// toolchain, and a caller passes [toolchain.Require] first.
func (adapter) Parse(ctx context.Context, dir string) error {
	sources, err := javaSources(dir)
	if err != nil {
		return err
	}
	driver, err := tool(dir, parseDriver, parseSource)
	if err != nil {
		return err
	}
	_, err = run(ctx, dir, javaBinary, append([]string{driver}, sources...)...)
	return err
}

// TypeCheck compiles every Java source with javac into the classes
// directory, which is Java's type check.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	sources, err := javaSources(dir)
	if err != nil {
		return err
	}
	args := []string{"-d", filepath.Join(dir, classesDir), "-proc:none", "-encoding", "UTF-8"}
	_, err = run(ctx, dir, javacBinary, append(args, sources...)...)
	return err
}

// RunTests compiles the project and runs every test class, a
// top-level class whose simple name ends in Test, by invoking its
// main method: a return passes and a throw fails, because the JDK
// includes no test framework and a scratch project installs none. A
// test class without a main method fails, naming the missing method.
// A project that does not compile returns the compiler's error, and
// one without a test class returns an empty report, which fails the
// kernel's assertion.
func (a adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return toolchain.TestReport{}, err
	}
	classes := filepath.Join(dir, classesDir)
	var tests []string
	err := filepath.WalkDir(classes, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, testSuffix+classExt) ||
			strings.Contains(d.Name(), nestedMark) {

			return err
		}
		rel, err := filepath.Rel(classes, strings.TrimSuffix(path, classExt))
		if err != nil {
			return fmt.Errorf("testing: %w", err)
		}
		tests = append(tests, strings.ReplaceAll(filepath.ToSlash(rel), "/", "."))
		return nil
	})
	if err != nil {
		return toolchain.TestReport{}, err
	}
	if len(tests) == 0 {
		return toolchain.TestReport{}, nil
	}
	driver, err := tool(dir, testsDriver, testsSource)
	if err != nil {
		return toolchain.TestReport{}, err
	}
	out, err := run(ctx, dir, javaBinary, append([]string{"-cp", classes, driver}, tests...)...)
	report := tally(out)
	if err != nil && report.Failed == 0 {
		// java exited with an error and printed no fail line.
		return report, err
	}
	return report, nil
}

// Satisfies asks javac whether one type is assignable to one
// contract: a probe class returns a value of the type as the
// contract, compiled against the project's classes. Both are type
// names as Java spells them, fully qualified where the class is in a
// package, such as demo.Row and java.io.Closeable. A probe that javac
// refuses with incompatible types alone reports false. Any other
// refusal returns an error, and so does a project that does not
// compile without the probe.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("the project does not compile, so nothing can be asked of it: %w", err)
	}
	source := "final class EidosProbe {\n    static " + contract + " probe(" + typeName +
		" value) {\n        return value;\n    }\n}\n"
	probe, err := tool(dir, probeFile, source)
	if err != nil {
		return false, err
	}
	out, err := run(ctx, dir, javacBinary, "-XDrawDiagnostics", "-proc:none",
		"-cp", filepath.Join(dir, classesDir), "-d", filepath.Join(dir, toolDir, probeDir), probe)
	if err == nil {
		return true, nil
	}
	if refused := strings.Count(out, incompatible); refused > 0 && strings.Count(out, errorKey) == refused {
		return false, nil
	}
	return false, fmt.Errorf("the probe does not compile for a reason other than the contract: %w", err)
}

// javaSources returns every Java source of the project, the
// harness's own directories left out, and returns an error for a
// project without one.
func javaSources(dir string) ([]string, error) {
	var sources []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == classesDir || d.Name() == toolDir) {
			return filepath.SkipDir
		}
		if !d.IsDir() && filepath.Ext(path) == java.Extension {
			sources = append(sources, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("testing: the laid-out project has no Java file")
	}
	return sources, nil
}

// tool writes one of the harness's own sources into the tool
// directory and returns its path.
func tool(dir, name, source string) (string, error) {
	if err := write(dir, toolDir+"/"+name, []byte(source)); err != nil {
		return "", err
	}
	return filepath.Join(dir, toolDir, name), nil
}

// write writes one file under the scratch directory, and returns an
// error for a path that climbs out of it.
func write(dir, path string, body []byte) error {
	target := filepath.Join(dir, filepath.FromSlash(path))
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return fmt.Errorf("testing: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("the generated path %q climbs out of the scratch project", path)
	}
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return fmt.Errorf("testing: %w", err)
	}
	if err := os.WriteFile(target, body, filePerm); err != nil {
		return fmt.Errorf("testing: %w", err)
	}
	return nil
}

// run runs one toolchain binary in dir under ctx and returns its
// combined output. A failure wraps that output, so the error contains
// what the tool reported. The run is bounded by [runTimeout] as well,
// and a run that ctx or the bound ends returns an error that wraps the
// context's error.
func run(ctx context.Context, dir, binary string, args ...string) (string, error) {
	bounded, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	cmd := exec.CommandContext(bounded, binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return string(out), fmt.Errorf(
				"testing: %s %s stopped with its caller: %w", binary, strings.Join(args, " "), ctx.Err(),
			)
		}
		if bounded.Err() != nil {
			return string(out), fmt.Errorf(
				"testing: %s %s did not return within %s: %w",
				binary, strings.Join(args, " "), runTimeout, bounded.Err(),
			)
		}
		return string(out), fmt.Errorf("%s %s: %w\n%s", binary, strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// tally counts the test driver's pass and fail lines.
func tally(stream string) toolchain.TestReport {
	report := toolchain.TestReport{Output: stream}
	for line := range strings.Lines(stream) {
		switch {
		case strings.HasPrefix(line, passLine):
			report.Passed++
		case strings.HasPrefix(line, failLine):
			report.Failed++
		}
	}
	return report
}
