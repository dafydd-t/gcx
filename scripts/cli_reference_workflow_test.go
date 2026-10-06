package scripts_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublishCLIReference(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mode       string
		wantCommit bool
		wantPR     string
		wantError  bool
	}{
		{name: "new PR", wantCommit: true, wantPR: "pr create"},
		{name: "rolling PR", mode: "open", wantCommit: true, wantPR: "pr edit"},
		{name: "only CLI changed", mode: "cli-only", wantCommit: true, wantPR: "pr create"},
		{name: "only config changed", mode: "config-only", wantCommit: true, wantPR: "pr create"},
		{name: "unchanged", mode: "unchanged"},
		{name: "outdated release", mode: "outdated"},
		{name: "recycle merged branch", mode: "merged", wantCommit: true, wantPR: "pr create"},
		{name: "commit failure", mode: "fail", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			page := filepath.Join(dir, "cli-reference.md")
			require.NoError(t, os.WriteFile(page, []byte("new page\n"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "configuration.md"), []byte("new config\n"), 0600))
			mock := `#!/usr/bin/env bash
set -euo pipefail
echo "$*" >> "$MOCK_DIR/calls"
case "$1 $2" in
  'pr list') if [[ "$MOCK_MODE" == open ]]; then echo 123; fi ;;
  'pr create'|'pr edit') ;;
  'api graphql')
    if [[ "$MOCK_MODE" == fail ]]; then exit 1; fi
    cp "$4" "$MOCK_DIR/commit.json"
    echo '{"data":{"createCommitOnBranch":{"commit":{"oid":"new-head"}}}}'
    ;;
  'api repos/example/gcx/releases/latest')
    if [[ "$MOCK_MODE" == outdated ]]; then echo v1.3.0; else echo v1.2.3; fi ;;
  'api repos/example/gcx/contents/docs/sources/cli-reference.md'*)
    if [[ "$MOCK_MODE" == unchanged || "$MOCK_MODE" == config-only ]]; then echo 'new page'; else echo 'old page'; fi ;;
  'api repos/example/gcx/contents/docs/sources/configuration.md'*)
    if [[ "$MOCK_MODE" == unchanged || "$MOCK_MODE" == cli-only ]]; then echo 'new config'; else echo 'old config'; fi ;;
  'api repos/example/gcx/git/matching-refs/'*)
    if [[ "$MOCK_MODE" == merged ]]; then echo 1; else echo 0; fi ;;
  'api repos/example/gcx/git/ref/'*) echo expected-head ;;
  'api repos/example/gcx/git/refs'|'api repos/example/gcx/merges') ;;
  *) echo "Unexpected gh call: $*" >&2; exit 2 ;;
esac
`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(mock), 0600))
			require.NoError(t, os.Chmod(filepath.Join(dir, "gh"), 0700))
			cmd := exec.CommandContext(t.Context(), "bash", "publish-cli-reference.sh", "v1.2.3", "source-sha", dir)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_REPO=example/gcx", "MOCK_DIR="+dir, "MOCK_MODE="+tc.mode)
			output, err := cmd.CombinedOutput()
			if tc.wantError {
				require.Error(t, err, string(output))
			} else {
				require.NoError(t, err, string(output))
			}
			calls, err := os.ReadFile(filepath.Join(dir, "calls"))
			require.NoError(t, err)
			if tc.wantPR != "" {
				require.Contains(t, string(calls), tc.wantPR)
			} else {
				require.NotContains(t, string(calls), "pr create")
				require.NotContains(t, string(calls), "pr edit")
			}
			if !tc.wantCommit {
				if !tc.wantError {
					require.NotContains(t, string(calls), "api graphql")
				}
				return
			}
			data, err := os.ReadFile(filepath.Join(dir, "commit.json"))
			require.NoError(t, err)
			var request struct {
				Variables struct {
					Input struct {
						ExpectedHeadOID string `json:"expectedHeadOid"`
						FileChanges     struct {
							Additions []struct {
								Path     string `json:"path"`
								Contents string `json:"contents"`
							} `json:"additions"`
						} `json:"fileChanges"`
					} `json:"input"`
				} `json:"variables"`
			}
			require.NoError(t, json.Unmarshal(data, &request))
			require.Equal(t, "expected-head", request.Variables.Input.ExpectedHeadOID)
			require.Len(t, request.Variables.Input.FileChanges.Additions, 2)
			require.Equal(t, "docs/sources/cli-reference.md", request.Variables.Input.FileChanges.Additions[0].Path)
			require.Equal(t, "bmV3IHBhZ2UK", request.Variables.Input.FileChanges.Additions[0].Contents)
			require.Equal(t, "docs/sources/configuration.md", request.Variables.Input.FileChanges.Additions[1].Path)
			require.Equal(t, "bmV3IGNvbmZpZwo=", request.Variables.Input.FileChanges.Additions[1].Contents)
		})
	}
}
