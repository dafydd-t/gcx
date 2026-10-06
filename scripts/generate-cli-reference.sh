#!/usr/bin/env bash
set -euo pipefail

# Run the current renderer against the release's command tree and metadata.
if [[ $# != 3 ]]; then
  echo "Usage: $0 <release-source-directory> <release-tag> <output-file>" >&2
  exit 1
fi
source_dir=$(cd "$1" && pwd)
version=$2
output=$(cd "$(dirname "$3")" && pwd)/$(basename "$3")
scripts_dir=$(cd "$(dirname "$0")" && pwd)
temp_dir=$(mktemp -d)
renderer_dir=$(mktemp -d "$source_dir/scripts/reference-renderer.XXXXXX")
trap 'rm -rf "$temp_dir" "$renderer_dir"' EXIT
cp "$scripts_dir"/cli-reference/*.go "$renderer_dir/"
cd "$source_dir"
export GCX_AGENT_MODE=false CGO_ENABLED=0
# Prevent local credentials or defaults from affecting the reference.
export GCX_CONFIG="$temp_dir/config.yaml"
go run ./scripts/config-reference "$temp_dir/config"
go run ./scripts/env-vars-reference "$temp_dir/env"
go run "$renderer_dir" --version "$version" --config "$temp_dir/config/index.md" --env "$temp_dir/env/index.md" --output "$output"
