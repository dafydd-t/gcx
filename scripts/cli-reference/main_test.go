package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/grafana/gcx/cmd/gcx/root"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRenderCommands(t *testing.T) {
	for _, tc := range []struct {
		name       string
		hidden     bool
		deprecated string
	}{
		{name: "public"},
		{name: "hidden", hidden: true},
		{name: "deprecated", deprecated: "Use gcx new instead."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "gcx"}
			cmd.PersistentFlags().String("context", "", "Context to use")
			group := &cobra.Command{Use: "group"}
			group.PersistentFlags().String("project", "", "Project ID")
			leaf := &cobra.Command{Use: "get ID", Aliases: []string{"show"}, Hidden: tc.hidden, Deprecated: tc.deprecated, Run: func(*cobra.Command, []string) {}}
			leaf.Flags().String("context", "local", "Local context override")
			leaf.Flags().String("internal", "", "Hidden flag")
			require.NoError(t, leaf.Flags().MarkHidden("internal"))
			group.AddCommand(leaf)
			cmd.AddCommand(group)
			page := render(cmd, "v1.2.3", "# Config\n\n```yaml\n# schema comment\nversion: int\n```", "# Env\n\n## `GCX_TEST`\n\nTest variable")
			require.Contains(t, page, "gcx **v1.2.3**")
			require.Contains(t, page, "### `GCX_TEST`")
			require.Contains(t, page, "# schema comment")
			require.NotContains(t, page, "--internal")
			if tc.hidden {
				require.NotContains(t, page, "gcx group get")
				return
			}
			require.Contains(t, page, "### `gcx group get` {#gcx-group-get}")
			require.Contains(t, page, "gcx group get ID [flags]")
			require.Contains(t, page, "**Aliases:** `show`")
			_, leafPage, ok := strings.Cut(page, "### `gcx group get`")
			require.True(t, ok)
			require.Contains(t, leafPage, "Inherits `--project` from [`gcx group`](#gcx-group-flags)")
			require.NotContains(t, leafPage, "Inherits `--context`")
			if tc.deprecated != "" {
				require.Contains(t, leafPage, "**Deprecated:** "+tc.deprecated)
			}
		})
	}
}

func TestFullCommandCoverageAndLinks(t *testing.T) {
	t.Setenv("GCX_AGENT_MODE", "false")
	cmd := root.Command("v1.2.3")
	cmd.Use = "gcx"
	page := render(cmd, "v1.2.3", "# Config\n\n```yaml\nversion: int\n```", "# Env\n\n## `GCX_TEST`\n\nTest variable")
	ids := map[string]bool{"commands": true, "environment-variables": true, "configuration": true}
	for _, match := range regexp.MustCompile(`\{#([^}]+)\}`).FindAllStringSubmatch(page, -1) {
		require.False(t, ids[match[1]], "duplicate anchor %s", match[1])
		ids[match[1]] = true
	}
	for _, match := range regexp.MustCompile(`\]\(#([^)]+)\)`).FindAllStringSubmatch(page, -1) {
		require.True(t, ids[match[1]], "broken anchor %s", match[1])
	}
	var check func(*cobra.Command)
	check = func(command *cobra.Command) {
		if command.Hidden {
			return
		}
		require.Equal(t, 1, strings.Count(page, "### `"+command.CommandPath()+"` {#"))
		for _, child := range command.Commands() {
			check(child)
		}
	}
	check(cmd)
	require.Equal(t, page, render(cmd, "v1.2.3", "# Config\n\n```yaml\nversion: int\n```", "# Env\n\n## `GCX_TEST`\n\nTest variable"))
}

func TestDateDefaults(t *testing.T) {
	cmd := &cobra.Command{Use: "gcx"}
	cmd.Flags().String("date", "2026-10-06", "Report date")
	page := render(cmd, "v1.2.3", "# Config", "# Env")
	require.Contains(t, page, `(default "YYYY-MM-DD")`)
	require.NotContains(t, page, "2026-10-06")
}

func TestSupplementEnvironment(t *testing.T) {
	env := "# Env\n\n## `GRAFANA_SERVER`\n\nServer URL"
	guide := "| `GRAFANA_SERVER` | context | duplicate |\n" +
		"| `GCX_CONFIG` | global | Config file override |\n" +
		"| `GRAFANA_PROVIDER_SLO_TOKEN` | slo | token |\n" +
		"| `GCX_AGENT_MODE` | opt-in/out | Enable agent mode |\n" +
		"| `OPENCODE` | opencode | Truthy value activates agent mode |"
	page := supplementEnvironment(env, guide)
	require.Equal(t, 1, strings.Count(page, "## `GRAFANA_SERVER`"))
	require.Contains(t, page, "## `GCX_CONFIG`\n\nConfig file override")
	require.Contains(t, page, "Overrides the slo provider's token configuration.")
	require.Contains(t, page, "## `GCX_AGENT_MODE`")
	require.Contains(t, page, "## `OPENCODE`\n\nTruthy value activates agent mode")
}
