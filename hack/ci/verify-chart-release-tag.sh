#!/usr/bin/env bash
#
# Copyright 2026 ptrvsrg.
#
# Licensed under the Apache License, Version 2.0 (the License);
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an 'AS IS' BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Validates GITHUB_REF (refs/tags/<name>) against charts/<chart>/Chart.yaml version.
# Used by: .github/workflows/release-chart.yml

set -euo pipefail

SCRIPT_NAME=$(basename "$0")

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [--help]

  Validate that a Git tag matches the "version" field in the corresponding Helm
  chart Chart.yaml. Intended for chart release tags in CI (GITHUB_REF is set by
  GitHub Actions).

Environment:
  GITHUB_REF   Required. Must be refs/tags/<tag>, e.g. refs/tags/csi-driver-ipfs-0.1.0
  REPO_ROOT    Optional. Repository root (default: git toplevel or current dir)

Tag format:
  <chart-name>-<semver>
  Charts: csi-driver-ipfs, ipfs-cluster
  Examples: chart/csi-driver-ipfs/v0.1.0, chart/ipfs-cluster/v1.2.3-rc.1

Local test:
  GITHUB_REF=refs/tags/chart/csi-driver-ipfs/v0.1.0 REPO_ROOT="\$(git rev-parse --show-toplevel)" ${SCRIPT_NAME}
EOF
}

die() {
  echo "${SCRIPT_NAME}: $*" >&2
  exit 1
}

# Resolve repository root for Chart.yaml paths.
resolve_repo_root() {
  local root
  if [[ -n "${REPO_ROOT:-}" ]]; then
    root="$REPO_ROOT"
  else
    root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
  fi
  if [[ ! -d "$root/charts" ]]; then
    die "REPO_ROOT must contain charts/ (got: $root)"
  fi
  printf '%s' "$root"
}

# Parse GITHUB_REF; prints "chart|version" (single line, pipe is not in chart names).
parse_tag_from_ref() {
  local ref="$1"
  local tag chart version prefix

  [[ "$ref" =~ ^refs/tags/ ]] || die "GITHUB_REF must be a tag ref, got: ${ref:-empty}"
  tag="${ref#refs/tags/}"

  chart=""
  version=""
  for c in csi-driver-ipfs ipfs-cluster; do
    prefix="chart/${c}/v"
    if [[ "$tag" == "${prefix}"* ]]; then
      chart="$c"
      version="${tag#"${prefix}"}"
      break
    fi
  done

  if [[ -z "$chart" || -z "$version" ]]; then
    die "Tag must be chart/<chart_name>/v<semver>, got: $tag"
  fi

  printf '%s|%s' "$chart" "$version"
}

chart_yaml_version() {
  local chart_file="$1"
  awk '
    /^version:/ {
      sub(/^version:[[:space:]]+/, "")
      gsub(/^["'\'']|["'\'']$/, "")
      print
      exit
    }' "$chart_file"
}

# Compare Chart.yaml .version to expected semver from the tag.
verify_chart_version() {
  local repo_root="$1"
  local chart="$2"
  local expected_version="$3"
  local chart_file actual

  chart_file="${repo_root}/charts/${chart}/Chart.yaml"
  [[ -f "$chart_file" ]] || die "Missing $chart_file"

  actual="$(chart_yaml_version "$chart_file")"
  [[ -n "$actual" ]] || die "Could not read version from ${chart_file}"
  if [[ "$actual" != "$expected_version" ]]; then
    die "Chart.yaml version (${actual}) does not match tag version (${expected_version}) for chart ${chart}. Bump charts/${chart}/Chart.yaml before tagging."
  fi

  echo "OK: chart=${chart} version=${expected_version} matches Chart.yaml"
}

main() {
  case "${1:-}" in
  -h | --help)
    usage
    return 0
    ;;
  esac

  if [[ -n "${1:-}" ]]; then
    die "unexpected argument: $1 (use --help)"
  fi

  local ref="${GITHUB_REF:-}"
  [[ -n "$ref" ]] || die "GITHUB_REF is not set"

  local repo_root parsed chart version
  repo_root="$(resolve_repo_root)"
  parsed="$(parse_tag_from_ref "$ref")"
  chart="${parsed%%|*}"
  version="${parsed#*|}"

  verify_chart_version "$repo_root" "$chart" "$version"
}

main "$@"
