// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
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

// explainedFileEvent is an explain.file event of JSON output.
type explainedFileEvent struct {
	Path         string       `json:"path"`
	Plan         string       `json:"plan"`
	Plugins      []string     `json:"plugins"`
	Contributors []matchEvent `json:"contributors"`
	Workspace    string       `json:"workspace"`
}

// readEvent is one read of a record or of a claim in JSON output.
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
	Bucket    int         `json:"bucket"`
	Plugin    string      `json:"plugin"`
	Pos       string      `json:"pos"`
	Derived   []readEvent `json:"derived"`
}

// recordEvent is an explain.record event of JSON output.
type recordEvent struct {
	Kind     string         `json:"kind"`
	Check    string         `json:"check"`
	Findings []findingEvent `json:"findings"`
	Reads    []readEvent    `json:"reads"`
}

// Explain writes what the last run recorded about a target, and runs
// nothing.
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

		t.Run("writes the records of the group and of the contributors of a file", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), mirrorPath, jsonFlag)
			assert.Equal(t, kinds(t, stdout),
				[]string{workspace.RecordGroup.String(), workspace.RecordInvocation.String()},
				"the group comes before its contributor")
		})

		t.Run("writes the claim on the fact of a declaration", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget, jsonFlag)
			assert.Equal(t, decoded[claimEvent](t, stdout, eventExplainClaim), []claimEvent{{
				Key: string(flagKey), Subject: storeTarget, Value: true, Winner: true,
				Authority: meta.AuthorityPlugin.String(), Bucket: 1, Plugin: string(flaggerName),
				Derived: []readEvent{{Subject: storeTarget, Key: string(flagKey)}},
			}}, "the claim of the flagger derives from its read of the flag")
		})

		t.Run("writes the records that read a declaration", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget, jsonFlag)
			got := kinds(t, stdout)
			expect.Contains(t, got, workspace.RecordCheck.String(), "the lister looked the store up")
			expect.Contains(t, got, workspace.RecordInvocation.String(), "the plugins ran on the store")
		})

		t.Run("writes the kind of a read of a kind", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, source), storeTarget, jsonFlag)
			records := decoded[recordEvent](t, stdout, eventExplainRecord)
			i := slices.IndexFunc(records, func(r recordEvent) bool { return r.Check == string(listerName) })
			assert.NotEqual(t, i, -1, "the output has the record of the lister")
			assert.Contains(t, records[i].Reads,
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

		t.Run("writes the records that reported a finding", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, noisySource), noisyTarget, jsonFlag)
			got := decoded[recordEvent](t, stdout, eventExplainRecord)
			assert.Length(t, got, 1, "the mirror reported the finding")
			expect.Equal(t, got[0].Kind, workspace.RecordInvocation.String(), "an invocation reported it")
			expect.Equal(t, got[0].Findings, []findingEvent{{
				Code: noted.String(), Severity: diag.SeverityWarning.String(), Pos: "svc/store.zz:2",
				Msg: "Noisy is noisy", Origin: "mirror",
			}}, "the record has the finding")
		})

		t.Run("writes the explanation as text", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdExplain, explained(t, noisySource+dropped), "fake:svc.Noisy")
			expect.Contains(t, stdout, "file svc/mirror.txt: plan mirrors, plugins mirror\n"+
				"  contributor mirror rule 0 fake:svc.Noisy\n  source fake:svc.Noisy\n",
				"the text has the file and its contributor")
			expect.Contains(t, stdout, "claim cli.flag on fake:svc.Noisy: "+
				"value dropped, winner true, authority directive, bucket 0, at svc/store.zz:3\n",
				"the text has the drop")
			expect.Contains(t, stdout, "\n  derived from subject fake:svc.Noisy, key cli.flag\n",
				"the text has the read that the stamp derives from")
			expect.Contains(t, stdout, "\n  svc/store.zz:2: warning "+noted.String()+": Noisy is noisy (mirror)\n",
				"the text has the finding of a record")
			expect.Contains(t, stdout, "\n  read declaration of fake:svc.Noisy\n",
				"the text has the read of a declaration")
			expect.Contains(t, stdout, "\n  read fact cli.flag of fake:svc.Noisy\n", "the text has the read of a fact")
			expect.Contains(t, stdout, "\n  read kind "+symbol.KindStruct.String()+"\n",
				"the text has the read of a kind")
		})

		t.Run("reads a path relative to the working directory", func(t *testing.T) {
			t.Parallel()

			root := explained(t, source)
			_, stdout, _ := invoke(t, described, cmdExplain, filepath.Join(root, "svc"), "mirror.txt", jsonFlag)
			got := decoded[explainedFileEvent](t, stdout, eventExplainFile)
			assert.Length(t, got, 1, "the path is the path of the mirror")
			assert.Equal(t, got[0].Path, mirrorPath, "the path is relative to the root")
		})

		t.Run("explains a path in the member of a list that contains it", func(t *testing.T) {
			t.Parallel()

			root := listed(t)
			ran(t, root)
			_, stdout, _ := invoke(t, compose, cmdExplain, root, "tools/mirror.txt", jsonFlag)
			got := decoded[explainedFileEvent](t, stdout, eventExplainFile)
			assert.Length(t, got, 1, "the member tools has the file")
			expect.Equal(t, got[0].Workspace, memberTool, "the event names the member")
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

// kinds returns the kind of each record event of JSON output, in the order
// of the output.
func kinds(t *testing.T, stdout string) []string {
	t.Helper()

	var out []string
	for _, r := range decoded[recordEvent](t, stdout, eventExplainRecord) {
		out = append(out, r.Kind)
	}
	return out
}
