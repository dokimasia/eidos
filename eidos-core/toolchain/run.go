// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunTimeout bounds one toolchain invocation. A compiler that has not
// returned after this time is a fault of the harness, and an unbounded run
// would stop a suite instead of failing one assertion.
const RunTimeout = 5 * time.Minute

// errRunTimeout is the cause of a run that RunTimeout ends. It wraps
// [context.DeadlineExceeded], as the error of any context with a deadline
// does.
var errRunTimeout = fmt.Errorf("toolchain: the run passed its bound of %s: %w", RunTimeout, context.DeadlineExceeded)

// Run runs binary with args in dir under ctx and returns the combined
// output of the binary. env lists the variables that the run sets on top of
// the environment of the process, and a nil env runs the binary in that
// environment unchanged. [RunTimeout] bounds the run.
//
// Error modes:
//   - a binary that exits with an error returns an error that wraps the
//     [*exec.ExitError] and contains the output;
//   - a binary that does not start returns an error that wraps the error
//     of os/exec, such as [exec.ErrNotFound];
//   - a run that ctx ends returns an error that wraps the error of ctx;
//   - a run that RunTimeout ends returns an error that wraps
//     [context.DeadlineExceeded].
func Run(ctx context.Context, dir string, env []string, binary string, args ...string) (string, error) {
	bounded, cancel := context.WithTimeoutCause(ctx, RunTimeout, errRunTimeout)
	defer cancel()

	cmd := exec.CommandContext(bounded, binary, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), nil
	}
	if bounded.Err() != nil {
		return string(out), fmt.Errorf("toolchain: %s %s stopped before it returned: %w",
			binary, strings.Join(args, " "), context.Cause(bounded))
	}
	return string(out), fmt.Errorf("toolchain: %s %s: %w\n%s", binary, strings.Join(args, " "), err, out)
}

// LookPaths reports whether every binary is on PATH. Where a binary is not,
// LookPaths returns false and a reason that names the first such binary,
// which is the reason that [Adapter.Available] returns.
func LookPaths(binaries ...string) (bool, string) {
	for _, binary := range binaries {
		if _, err := exec.LookPath(binary); err != nil {
			return false, fmt.Sprintf("%s is not on PATH: %v", binary, err)
		}
	}
	return true, ""
}
