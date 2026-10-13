// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// explain reads storeTarget as an identity, and flagTarget as the flag of
// the store at the line of its declaration. The composition does not
// register the key of ghostTarget.
const (
	storeTarget = "fake:svc.Store"
	flagTarget  = string(flagKey) + "@" + storePath + ":2"
	ghostTarget = "ghost.key@" + storePath + ":2"
)

// dropped is the scripted line of a meta directive that drops the flag of
// the struct above it.
const dropped = "+meta drop=" + string(flagKey) + "\n"

// The targets that no generation records: a path that no plan generates,
// and the identity of a struct that no source declares.
const (
	ungeneratedPath = "svc/none.txt"
	ghostIdentity   = "fake:ghost.Ghost"
)

// moved is the scripted line of the out directive of the kernel, which
// moves the mirror of the struct above it into the directory gen beside its
// source. movedPath is the path of the moved mirror.
const (
	moved     = "+out path=gen/\n"
	movedPath = "svc/gen/mirror.txt"
)

// noisyTarget is the finding of the mirror at the line of the struct Noisy.
var noisyTarget = noted.String() + "@" + storePath + ":2"

// generationEvent is the explain event of JSON output.
type generationEvent struct {
	Generation string `json:"generation"`
	Recorded   string `json:"recorded"`
}

// matchEvent is the match of an invocation in JSON output.
type matchEvent struct {
	Plugin  string `json:"plugin"`
	Rule    int    `json:"rule"`
	Subject string `json:"subject"`
}

// declarationEvent is an explain.declaration event of JSON output.
type declarationEvent struct {
	Subject     string          `json:"subject"`
	Pos         string          `json:"pos"`
	Declaration json.RawMessage `json:"declaration"`
	Member      string          `json:"member"`
}

// kindOf is the kind of the JSON form of a declaration.
type kindOf struct {
	Kind string `json:"kind"`
}

// explainedFileEvent is an explain.file event of JSON output.
type explainedFileEvent struct {
	Path         string       `json:"path"`
	Plan         string       `json:"plan"`
	Plugins      []string     `json:"plugins"`
	Contributors []matchEvent `json:"contributors"`
	Member       string       `json:"member"`
}

// routesEvent is the list of the routes of an explain.file event of JSON
// output.
type routesEvent struct {
	Routes []routeEvent `json:"routes"`
}

// routeEvent is the route of one unit of a file in JSON output.
type routeEvent struct {
	Plugin  string `json:"plugin"`
	Tag     string `json:"tag"`
	Package string `json:"package"`
	Key     string `json:"key"`
	Family  string `json:"family"`
	Policy  string `json:"policy"`
	Dir     string `json:"dir"`
	Name    string `json:"name"`
}

// readEvent is one read of an execution or of a claim in JSON output.
type readEvent struct {
	Grain   string `json:"grain"`
	Subject string `json:"subject"`
	Key     string `json:"key"`
	Kind    string `json:"kind"`
}

// claimEvent is an explain.claim event of JSON output.
type claimEvent struct {
	Key       string      `json:"key"`
	Subject   string      `json:"subject"`
	Value     any         `json:"value"`
	Winner    bool        `json:"winner"`
	Authority string      `json:"authority"`
	Turn      int         `json:"turn"`
	Plugin    string      `json:"plugin"`
	Pos       string      `json:"pos"`
	Derived   []readEvent `json:"derived"`
}

// executionEvent is an explain.execution event of JSON output.
type executionEvent struct {
	Kind     string         `json:"kind"`
	Check    string         `json:"check"`
	Findings []findingEvent `json:"findings"`
	Reads    []readEvent    `json:"reads"`
}

// TestExplain tests the command explain, which writes what the last run
// recorded about a target and runs nothing.
func TestExplain(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the generation that it reads", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), mirrorPath, jsonFlag)
			got := decoded[generationEvent](t, stdout, eventExplain)
			assert.Length(t, got, 1, "the output has one generation")
			expect.NotEmpty(t, got[0].Generation, "the event has the name of the generation")
			_, err := time.Parse(time.RFC3339, got[0].Recorded)
			expect.NoError(t, err, "the event has the time of the record in RFC 3339 form")
		})

		t.Run("writes the entry and the contributors of a generated file", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), mirrorPath, jsonFlag)
			assert.Equal(t, decoded[explainedFileEvent](t, stdout, eventExplainFile), []explainedFileEvent{{
				Path: mirrorPath, Plan: planName, Plugins: []string{"mirror"},
				Contributors: []matchEvent{{Plugin: "mirror", Subject: storeTarget}},
			}}, "the invocation of the mirror on the store contributed to the file")
		})

		t.Run("writes the route of the unit of a generated file", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), mirrorPath, jsonFlag)
			assert.Equal(t, decoded[routesEvent](t, stdout, eventExplainFile), []routesEvent{{Routes: []routeEvent{{
				Plugin: "mirror", Package: "fake:svc", Key: "svc",
				Family: plugin.RouteHandler.String(), Policy: plugin.RouteDefault.String(),
				Dir: plugin.RouteSource.String(), Name: plugin.RouteTarget.String(),
			}}}}, "the layout placed the unit of the mirror beside its source")
		})

		t.Run("writes no policy into the route of a unit that an out directive moves", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source+moved), movedPath, jsonFlag)
			assert.Equal(t, decoded[routesEvent](t, stdout, eventExplainFile), []routesEvent{{Routes: []routeEvent{{
				Plugin: "mirror", Package: "fake:svc", Key: "svc", Family: plugin.RouteHandler.String(),
				Dir: plugin.RouteOut.String(), Name: plugin.RouteTarget.String(),
			}}}}, "the path of the directive leaves the policy out of the route")
		})

		t.Run("writes the declaration of an identity", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget, jsonFlag)
			got := decoded[declarationEvent](t, stdout, eventExplainDeclaration)
			assert.Length(t, got, 1, "the identity names one declaration")
			expect.Equal(t, got[0].Subject, storeTarget, "the event has the identity of the declaration")
			expect.Equal(t, got[0].Pos, storePath+":2", "the event has the position of the declaration")
			var kind kindOf
			assert.NoError(t, json.Unmarshal(got[0].Declaration, &kind), "the declaration is a JSON object")
			expect.Equal(t, kind.Kind, symbol.KindStruct.String(), "the declaration has the kind of the store")
		})

		t.Run("writes the declaration of an identity as text", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget)
			want := "declaration " + storeTarget + " at " + storePath + ":2\n" +
				"  {\n    \"kind\": \"" + symbol.KindStruct.String() + "\",\n"
			assert.Contains(t, stdout, want, "the text has the line of the declaration and its JSON form below it")
		})

		t.Run("returns StatusFailed for a target that the generation records nothing about", func(t *testing.T) {
			t.Parallel()

			status, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), ungeneratedPath, jsonFlag)
			expect.Equal(t, status, cli.StatusFailed, "an empty explanation fails explain")
			generations := decoded[generationEvent](t, stdout, eventExplain)
			assert.Length(t, generations, 1, "the output has the generation that explain read")
			errs := decoded[errorEvent](t, stdout, eventError)
			assert.Length(t, errs, 1, "the output has one error")
			expect.Equal(t, errs[0].Msg, "cli: explain: the generation "+generations[0].Generation+
				" records nothing about "+ungeneratedPath, "the error names the generation and the target")
		})

		t.Run("returns StatusOK for an identity that one member of a list records", func(t *testing.T) {
			t.Parallel()

			root := listed(t)
			ran(t, root)
			status, _, stderr := invoke(t, compose, cmdExplain, root, "fake:store.Store")
			expect.Equal(t, status, cli.StatusOK, "the member svc records the store")
			expect.Empty(t, stderr, "explain writes no error for the member tools")
		})

		t.Run("writes an error for each member of a list that records nothing about the target", func(t *testing.T) {
			t.Parallel()

			root := listed(t)
			ran(t, root)
			status, stdout, _ := invoke(t, compose, cmdExplain, root, ghostIdentity, jsonFlag)
			expect.Equal(t, status, cli.StatusFailed, "no member records the identity")
			errs := decoded[errorEvent](t, stdout, eventError)
			assert.Length(t, errs, 2, "each member has one error")
			expect.Equal(t, errs[0].Member, memberSvc, "the first error names the first member")
			expect.Equal(t, errs[1].Member, memberTool, "the second error names the second member")
		})

		t.Run("writes the group and the contributors of a file", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), mirrorPath, jsonFlag)
			assert.Equal(t, kinds(t, stdout),
				[]string{workspace.ExecutionGroup.String(), workspace.ExecutionInvocation.String()},
				"the group comes before its contributor")
		})

		t.Run("writes the claim on the fact of a declaration", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget, jsonFlag)
			assert.Equal(t, decoded[claimEvent](t, stdout, eventExplainClaim), []claimEvent{
				{
					Key: string(target.NameKey()), Subject: storeTarget, Value: "Store", Winner: true,
					Authority: meta.AuthorityPlugin.String(), Turn: 1, Plugin: string(loweringName),
					Derived: []readEvent{{Subject: storeTarget}},
				},
				{
					Key: string(flagKey), Subject: storeTarget, Value: true, Winner: true,
					Authority: meta.AuthorityPlugin.String(), Turn: 2, Plugin: string(flaggerName),
					Derived: []readEvent{{Subject: storeTarget, Key: string(flagKey)}},
				},
			}, "the lowering entry claims the name of the store, and the flagger claims the flag that it read")
		})

		t.Run("writes the executions that read a declaration", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget, jsonFlag)
			expect.That(t, kinds(t, stdout)).
				Contains(workspace.ExecutionCheck.String(), "the lister looked the store up").
				Contains(workspace.ExecutionInvocation.String(), "the plugins ran on the store")
		})

		t.Run("writes the kind of a read of a kind", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget, jsonFlag)
			executions := decoded[executionEvent](t, stdout, eventExplainExecution)
			i := slices.IndexFunc(executions, func(r executionEvent) bool { return r.Check == string(listerName) })
			assert.NotEqual(t, i, -1, "the output has the execution of the lister")
			assert.Contains(t, executions[i].Reads,
				readEvent{Grain: workspace.ReadKind.String(), Kind: symbol.KindStruct.String()},
				"the lister ranged over the structs")
		})

		t.Run("writes a drop at its position as the claim that ranks first", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source+dropped), flagTarget, jsonFlag)
			got := decoded[claimEvent](t, stdout, eventExplainClaim)
			assert.Length(t, got, 2, "the drop and the stamp claim the flag")
			expect.Equal(t, got[0].Authority, meta.AuthorityDirective.String(), "the drop ranks first")
			expect.Nil(t, got[0].Value, "a drop has no value")
			expect.Equal(t, got[0].Pos, "svc/store.zz:3", "the drop is at its directive")
			expect.False(t, got[1].Winner, "the stamp ranks second")
		})

		t.Run("writes the executions that reported a finding", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, noisySource), noisyTarget, jsonFlag)
			got := decoded[executionEvent](t, stdout, eventExplainExecution)
			assert.Length(t, got, 1, "the mirror reported the finding")
			expect.Equal(t, got[0].Kind, workspace.ExecutionInvocation.String(), "an invocation reported it")
			expect.Equal(t, got[0].Findings, []findingEvent{{
				Code: noted.String(), Severity: diag.SeverityWarning.String(), Pos: "svc/store.zz:2",
				Msg: "Noisy is noisy", Origin: "mirror",
			}}, "the execution has the finding")
		})

		t.Run("writes the explanation as text", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, noisySource+dropped), "fake:svc.Noisy")
			expect.That(t, stdout).
				Contains("file svc/mirror.txt: plan mirrors, plugins mirror\n"+
					"  route plugin mirror, key svc: family handler, policy default, dir source, name target\n"+
					"  contributor mirror rule 0 fake:svc.Noisy\n  source fake:svc.Noisy\n",
					"the text has the file, the route of its unit and its contributor").
				Contains("claim cli.flag on fake:svc.Noisy: "+
					"value dropped, winner true, authority directive, turn 0, at svc/store.zz:3\n",
					"the text has the drop").
				Contains("\n  derived from subject fake:svc.Noisy, key cli.flag\n",
					"the text has the read that the stamp derives from").
				Contains("\n  svc/store.zz:2: warning "+noted.String()+": Noisy is noisy (mirror)\n",
					"the text has the finding of an execution").
				Contains("\n  read declaration of fake:svc.Noisy\n", "the text has the read of a declaration").
				Contains("\n  read fact cli.flag of fake:svc.Noisy\n", "the text has the read of a fact").
				Contains("\n  read kind "+symbol.KindStruct.String()+"\n", "the text has the read of a kind")
		})

		t.Run("reads a path relative to the working directory", func(t *testing.T) {
			t.Parallel()

			root := explained(t, source)
			_, stdout, _ := invoke(t, described, cmdExplain, filepath.Join(root, "svc"), "mirror.txt", jsonFlag)
			got := decoded[explainedFileEvent](t, stdout, eventExplainFile)
			assert.Length(t, got, 1, "the path is the path of the mirror")
			assert.Equal(t, got[0].Path, mirrorPath, "the path is relative to the root")
		})

		t.Run("reads an absolute path through a symbolic link to the root", func(t *testing.T) {
			t.Parallel()

			root := explained(t, source)
			alias := filepath.Join(t.TempDir(), "alias")
			assert.NoError(t, os.Symlink(root, alias), "the alias of the root is created")
			_, stdout, _ := invoke(t, described, cmdExplain, root, filepath.Join(alias, mirrorPath), jsonFlag)
			got := decoded[explainedFileEvent](t, stdout, eventExplainFile)
			assert.Length(t, got, 1, "the path through the alias is the path of the mirror")
			assert.Equal(t, got[0].Path, mirrorPath, "the path is relative to the root")
		})

		t.Run("explains a path in the member of a list that contains it", func(t *testing.T) {
			t.Parallel()

			root := listed(t)
			ran(t, root)
			_, stdout, _ := invoke(t, compose, cmdExplain, root, "tools/mirror.txt", jsonFlag)
			got := decoded[explainedFileEvent](t, stdout, eventExplainFile)
			assert.Length(t, got, 1, "the member tools has the file")
			expect.Equal(t, got[0].Member, memberTool, "the event names the member")
			expect.Equal(t, got[0].Path, "mirror.txt", "the path is relative to the root of the member")
		})

		t.Run("returns StatusFailed before a run wrote a generation", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdExplain, stored(t, nil), mirrorPath)
			expect.Equal(t, status, cli.StatusFailed, "no run wrote a generation")
			expect.HasPrefix(t, stderr, "error: "+workspace.ErrNoGeneration.Error()+": ", "explain writes the error")
		})

		t.Run("returns StatusUsage without a target", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdExplain, stored(t, nil))
			expect.Equal(t, status, cli.StatusUsage, "explain takes one target")
			expect.HasPrefix(t, stderr, "error: cli: explain: the command takes one target, and has 0 arguments\n",
				"explain writes the error")
		})

		t.Run("returns StatusUsage for a key that the composition does not register", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdExplain, stored(t, nil), ghostTarget)
			expect.Equal(t, status, cli.StatusUsage, "the target is a usage error")
			expect.Contains(t, stderr, `"ghost.key" is neither a diagnostic code nor a key`, "explain writes the error")
		})

		t.Run("returns StatusUsage for a target outside the root", func(t *testing.T) {
			t.Parallel()

			outside := filepath.Join("..", "elsewhere.txt")
			status, _, stderr := invoke(t, compose, cmdExplain, stored(t, nil), outside)
			expect.Equal(t, status, cli.StatusUsage, "the target is a usage error")
			expect.HasPrefix(t, stderr,
				"error: cli: explain: the target "+outside+" is outside the root of every workspace\n",
				"explain writes the error")
		})

		t.Run("returns StatusUsage for a config error", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			status, _, _ := invoke(t, compose, cmdExplain, root, mirrorPath)
			assert.Equal(t, status, cli.StatusUsage, "the config error is a usage error")
		})
	})
}

// explained runs the described composition over a workspace whose store has
// the source src, and returns the root of the workspace.
func explained(t *testing.T, src string) string {
	t.Helper()

	root := stored(t, files.Tree{storePath: files.Text(src)})
	status, _, stderr := invoke(t, described, cmdRun, root)
	assert.Equal(t, status, cli.StatusOK, "the run succeeds: "+stderr)
	return root
}

// kinds returns the kind of each execution event of JSON output, in the
// order of the output.
func kinds(t *testing.T, stdout string) []string {
	t.Helper()

	executions := decoded[executionEvent](t, stdout, eventExplainExecution)
	out := make([]string, 0, len(executions))
	for _, r := range executions {
		out = append(out, r.Kind)
	}
	return out
}
