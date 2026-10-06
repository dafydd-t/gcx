package scripts_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
		{name: "unchanged", mode: "unchanged"},
		{name: "outdated release", mode: "outdated"},
		{name: "recycle merged branch", mode: "merged", wantCommit: true, wantPR: "pr create"},
		{name: "commit failure", mode: "fail", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			page := filepath.Join(dir, "page.md")
			require.NoError(t, os.WriteFile(page, []byte("new page\n"), 0600))
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
  'api repos/example/gcx/contents/'*)
    if [[ "$MOCK_MODE" == unchanged ]]; then echo 'new page'; else echo 'old page'; fi ;;
  'api repos/example/gcx/git/matching-refs/'*)
    if [[ "$MOCK_MODE" == merged ]]; then echo 1; else echo 0; fi ;;
  'api repos/example/gcx/git/ref/'*) echo expected-head ;;
  'api repos/example/gcx/git/refs'|'api repos/example/gcx/merges') ;;
  *) echo "Unexpected gh call: $*" >&2; exit 2 ;;
esac
`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(mock), 0600))
			require.NoError(t, os.Chmod(filepath.Join(dir, "gh"), 0700))
			cmd := exec.CommandContext(t.Context(), "bash", "publish-cli-reference.sh", "v1.2.3", "source-sha", page)
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
			require.Len(t, request.Variables.Input.FileChanges.Additions, 1)
			require.Equal(t, "docs/sources/cli-reference.md", request.Variables.Input.FileChanges.Additions[0].Path)
			require.Equal(t, "bmV3IHBhZ2UK", request.Variables.Input.FileChanges.Additions[0].Contents)
		})
	}
}

func TestSelectReferenceRelease(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/update-cli-reference.yaml")
	require.NoError(t, err)
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(data, &workflow))
	var script string
	for _, step := range workflow.Jobs["update"].Steps {
		if step.Name == "Select latest stable release" {
			script = step.Run
		}
	}
	require.NotEmpty(t, script)
	for _, tc := range []struct {
		name       string
		event      string
		tag        string
		sha        string
		wantOutput bool
	}{
		{name: "stable release", event: "workflow_run", tag: "v1.2.3", sha: "release-sha", wantOutput: true},
		{name: "old release", event: "workflow_run", tag: "v1.2.2", sha: "old-sha"},
		{name: "prerelease on same commit", event: "workflow_run", tag: "v1.3.0-rc.1", sha: "release-sha"},
		{name: "manual retry", event: "workflow_dispatch", wantOutput: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, output := range map[string]string{"gh": "v1.2.3", "git": "release-sha"} {
				path := filepath.Join(dir, name)
				require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho "+output+"\n"), 0600))
				require.NoError(t, os.Chmod(path, 0700))
			}
			outputPath := filepath.Join(dir, "output")
			require.NoError(t, os.WriteFile(outputPath, nil, 0600))
			cmd := exec.CommandContext(t.Context(), "bash", "-c", script)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_REPO=example/gcx", "GITHUB_OUTPUT="+outputPath, "EVENT_NAME="+tc.event, "RELEASE_TAG="+tc.tag, "RELEASE_COMMIT="+tc.sha)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, string(output))
			selected, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			if tc.wantOutput {
				require.Equal(t, "tag=v1.2.3\nsha=release-sha\n", string(selected))
			} else {
				require.Empty(t, selected)
			}
		})
	}
}
