#!/usr/bin/env bash
set -euo pipefail

: "${GH_REPO:?GH_REPO is required}"
if [[ $# != 3 ]]; then
  echo "Usage: $0 <release-tag> <source-commit> <generated-page>" >&2
  exit 1
fi
version=$1
source_commit=$2
page=$3
branch=docs/update-cli-reference
path=docs/sources/cli-reference.md
temp_dir=$(mktemp -d)
trap 'rm -rf "$temp_dir"' EXIT

# A newer release may have completed while this run was generating the page.
latest=$(gh api "repos/$GH_REPO/releases/latest" --jq .tag_name)
if [[ "$version" != "$latest" ]]; then
  echo "Skipping $version: latest stable release is $latest"
  exit 0
fi
pr=$(gh pr list --repo "$GH_REPO" --head "$branch" --base main --state open --json number --jq '.[0].number // empty')
base=main
if [[ -n "$pr" ]]; then
  base=$branch
fi
gh api "repos/$GH_REPO/contents/$path?ref=$base" -H 'Accept: application/vnd.github.raw+json' > "$temp_dir/previous.md"
if cmp -s "$page" "$temp_dir/previous.md"; then
  echo "CLI reference is already up to date"
  exit 0
fi

if [[ -z "$pr" ]]; then
  main_sha=$(gh api "repos/$GH_REPO/git/ref/heads/main" --jq .object.sha)
  refs=$(gh api "repos/$GH_REPO/git/matching-refs/heads/$branch" --jq ".[.[] | select(.ref == \"refs/heads/$branch\")] | length")
  if [[ "$refs" == 0 ]]; then
    gh api "repos/$GH_REPO/git/refs" -f "ref=refs/heads/$branch" -f "sha=$main_sha" > /dev/null
  else
    # Preserve the rolling branch history after a squash merge, without force pushes.
    gh api "repos/$GH_REPO/merges" -f "base=$branch" -f "head=$main_sha" > /dev/null
  fi
fi
head_sha=$(gh api "repos/$GH_REPO/git/ref/heads/$branch" --jq .object.sha)
base64 < "$page" | tr -d '\n' > "$temp_dir/content"
jq -n --arg repo "$GH_REPO" --arg branch "$branch" --arg head "$head_sha" \
  --arg path "$path" --arg title "docs: update CLI reference for $version" \
  --rawfile content "$temp_dir/content" '{
    query: "mutation($input: CreateCommitOnBranchInput!) { createCommitOnBranch(input: $input) { commit { oid } } }",
    variables: {input: {
      branch: {repositoryNameWithOwner: $repo, branchName: $branch},
      expectedHeadOid: $head,
      message: {headline: $title},
      fileChanges: {additions: [{path: $path, contents: $content}]}
    }}
  }' > "$temp_dir/commit.json"
# GitHub signs createCommitOnBranch commits as the authenticated bot.
gh api graphql --input "$temp_dir/commit.json" > /dev/null
cat > "$temp_dir/body.md" <<BODY
Update the CLI, environment-variable, and configuration reference for [$version](https://github.com/$GH_REPO/releases/tag/$version).

Generated from commit [$source_commit](https://github.com/$GH_REPO/commit/$source_commit). This rolling PR is updated when another stable release ships.

Click **Approve workflows to run** to start CI and the Grafana documentation preview, then review and merge manually.
BODY
if [[ -n "$pr" ]]; then
  gh pr edit "$pr" --repo "$GH_REPO" --title "docs: update CLI reference for $version" --body-file "$temp_dir/body.md"
else
  gh pr create --repo "$GH_REPO" --head "$branch" --base main --title "docs: update CLI reference for $version" --body-file "$temp_dir/body.md"
fi
