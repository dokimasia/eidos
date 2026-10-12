// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The spellings of the Java toolchain.
const (
	javacBinary = "javac"
	javaBinary  = "java"
	classExt    = ".class"
	// scratchPattern names the scratch directory of a laid-out project.
	scratchPattern = "eidos-java-*"
	// classesDir is the directory into which the project compiles, and
	// toolDir and probeDir are the directories of the drivers and the probe
	// of the harness. No generated path uses them, and the project leaves
	// them out of its own sources.
	classesDir = "eidos-classes"
	toolDir    = "eidos-tool"
	probeDir   = "eidos-probe"
	// testSuffix ends the simple name of a test class.
	testSuffix = "Test"
	// nestedMark joins the name of a nested class to the name of its host,
	// and a nested class is never a test by itself.
	nestedMark = "$"
	// packageSep separates the parts of the binary name of a class.
	packageSep = "."
	// incompatible is the raw key of javac for incompatible types, the one
	// refusal that makes a satisfaction probe report false.
	incompatible = "compiler.err.prob.found.req"
	// errorKey opens every raw error key that javac reports.
	errorKey = "compiler.err."
)

// The drivers that the harness runs through the source launcher of java,
// and the lines that the test driver prints.
const (
	parseDriver = "EidosParse.java"
	testsDriver = "EidosTests.java"
	probeFile   = "EidosProbe.java"
	passLine    = "eidos: pass "
	failLine    = "eidos: fail "
)

// parseSource reads every file that it receives through the parser of
// javac and does nothing after the parse, so a syntax error is told apart
// from a type error.
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

// testsSource runs the main method of every class that it receives and
// prints one line for each class. A return passes, and a throw fails.
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

// adapter drives javac and java over generated Java. It has no state, so
// one value serves every check.
type adapter struct{}

// New returns the Java toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [java.Lang], with which the messages of the assertions of
// the kernel start.
func (adapter) Lang() symbol.Lang { return java.Lang }

// Available reports whether javac and java are both on PATH, and names the
// first binary that it did not find.
func (adapter) Available() (bool, string) { return toolchain.LookPaths(javacBinary, javaBinary) }

// Layout writes the generated output as a source tree in a scratch
// directory, with every file at its own path. Java needs no build file,
// because javac compiles the sources that it receives. A write that fails,
// such as the write of a path that leaves the directory, returns an error
// and removes the directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	return toolchain.Scratch(scratchPattern, g, nil)
}

// Parse reads the syntax of every Java source through the parser of javac,
// which runs as a driver through the source launcher of java. It returns an
// error that contains the file and the line of every syntax error. A
// project without a Java file returns an error, because a parse of nothing
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
	_, err = toolchain.Run(ctx, dir, nil, javaBinary, append([]string{driver}, sources...)...)
	return err
}

// TypeCheck compiles every Java source with javac into the classes
// directory, which is the type check of Java.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	sources, err := javaSources(dir)
	if err != nil {
		return err
	}
	args := []string{"-d", filepath.Join(dir, classesDir), "-proc:none", "-encoding", "UTF-8"}
	_, err = toolchain.Run(ctx, dir, nil, javacBinary, append(args, sources...)...)
	return err
}

// RunTests compiles the project and runs every test class, a top-level
// class whose simple name ends in Test, by a call of its main method. A
// return passes and a throw fails, because the JDK includes no test
// framework and a scratch project installs none. A test class without a
// main method fails, and its line contains the missing method. A project
// that does not compile returns the error of the compiler. A project
// without a test class returns an empty report, which fails the assertion
// of the kernel.
func (a adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return toolchain.TestReport{}, err
	}
	classes := filepath.Join(dir, classesDir)
	found, err := toolchain.Sources(classes, []string{testSuffix + classExt})
	if err != nil {
		return toolchain.TestReport{}, err
	}
	var tests []string
	for _, class := range found {
		if strings.Contains(filepath.Base(class), nestedMark) {
			continue
		}
		rel, relErr := filepath.Rel(classes, strings.TrimSuffix(class, classExt))
		if relErr != nil {
			return toolchain.TestReport{}, fmt.Errorf("testing: %w", relErr)
		}
		tests = append(tests, strings.ReplaceAll(rel, string(filepath.Separator), packageSep))
	}
	if len(tests) == 0 {
		return toolchain.TestReport{}, nil
	}
	driver, err := tool(dir, testsDriver, testsSource)
	if err != nil {
		return toolchain.TestReport{}, err
	}
	out, err := toolchain.Run(ctx, dir, nil, javaBinary, append([]string{"-cp", classes, driver}, tests...)...)
	report := tally(out)
	if err != nil && report.Failed == 0 {
		// java exited with an error and printed no fail line.
		return report, err
	}
	return report, nil
}

// Satisfies asks javac whether one type is assignable to one contract. A
// probe class returns a value of the type as the contract, and javac
// compiles it against the classes of the project. Both are type names as
// Java spells them, fully qualified where the class is in a package, such
// as demo.Row and java.io.Closeable.
//
// A probe that javac refuses with incompatible types alone reports false.
// Satisfies returns an error for any other refusal, with the source of the
// probe, and for a project that does not compile without the probe.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("the project does not compile, so nothing can be asked of it: %w", err)
	}
	source := "final class EidosProbe {\n    static " + contract + " probe(" + typeName +
		" value) {\n        return value;\n    }\n}\n"
	var (
		out      string
		checkErr error
	)
	err := toolchain.Probe(dir, path.Join(toolDir, probeFile), source, func() error {
		out, checkErr = toolchain.Run(ctx, dir, nil, javacBinary, "-XDrawDiagnostics", "-proc:none",
			"-cp", filepath.Join(dir, classesDir), "-d", filepath.Join(dir, toolDir, probeDir),
			filepath.Join(dir, toolDir, probeFile))
		return nil
	})
	if err != nil {
		return false, err
	}
	if checkErr == nil {
		return true, nil
	}
	if refused := strings.Count(out, incompatible); refused > 0 && strings.Count(out, errorKey) == refused {
		return false, nil
	}
	return false, fmt.Errorf("the probe does not compile for a reason other than the contract: %w\nthe probe is:\n%s",
		checkErr, source)
}

// javaSources returns every Java source of the project without the
// directories of the harness. It returns an error for a project without a
// Java source.
func javaSources(dir string) ([]string, error) {
	sources, err := toolchain.Sources(dir, []string{java.Extension}, classesDir, toolDir)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("testing: the laid-out project has no Java file")
	}
	return sources, nil
}

// tool writes one of the sources of the harness into the tool directory
// and returns its path.
func tool(dir, name, source string) (string, error) {
	if err := toolchain.WriteFile(dir, path.Join(toolDir, name), []byte(source)); err != nil {
		return "", err
	}
	return filepath.Join(dir, toolDir, name), nil
}

// tally counts the pass and fail lines of the test driver.
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
