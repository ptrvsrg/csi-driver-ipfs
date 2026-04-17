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
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# For each chart under charts/ that has changes between BASE_SHA and HEAD_SHA,
# require that the "version" field in Chart.yaml differs between the two commits.
# Used by: .github/workflows/ci-dev-charts.yml

set -euo pipefail

SCRIPT_NAME=$(basename "$0")

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [--help]

  Fail if any file under charts/<name>/ changed between BASE_SHA and HEAD_SHA but
  the chart's Chart.yaml "version" value is unchanged.

Environment:
  BASE_SHA   Required. Base commit (e.g. PR base branch tip).
  HEAD_SHA   Required. Head commit (e.g. PR branch tip).

EOF
}

chart_version_at() {
  local commit="$1"
  local chart="$2"
  local blob
  # With pipefail, a missing path makes `git show` exit 128 and aborts the whole script.
  # BASE may not have charts/ yet (first import PR or new chart directory).
  if ! blob=$(git show "${commit}:${chart}/Chart.yaml" 2>/dev/null); then
    printf ''
    return 0
  fi
  printf '%s\n' "${blob}" | awk '
    /^version:/ {
      sub(/^version:[[:space:]]+/, "")
      gsub(/^["'\'']|["'\'']$/, "")
      print
      exit
    }'
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

if [[ -z "${BASE_SHA:-}" || -z "${HEAD_SHA:-}" ]]; then
  echo "${SCRIPT_NAME}: BASE_SHA and HEAD_SHA must be set" >&2
  exit 2
fi

if ! git cat-file -e "${BASE_SHA}^{commit}" 2>/dev/null; then
  echo "${SCRIPT_NAME}: not a commit: ${BASE_SHA}" >&2
  exit 2
fi
if ! git cat-file -e "${HEAD_SHA}^{commit}" 2>/dev/null; then
  echo "${SCRIPT_NAME}: not a commit: ${HEAD_SHA}" >&2
  exit 2
fi

changed_files=$(git diff --name-only "${BASE_SHA}" "${HEAD_SHA}" -- charts/ || true)
if [[ -z "$changed_files" ]]; then
  echo "No changes under charts/ — skipping version bump check."
  exit 0
fi

# Only Helm chart roots (directory with Chart.yaml at HEAD), not repo files like charts/README.md.
charts=$(
  printf '%s\n' "${changed_files}" |
    awk -F/ '$1 == "charts" && NF >= 2 { print $1"/"$2 }' |
    sort -u |
    while IFS= read -r chart; do
      [[ -z "${chart}" ]] && continue
      git cat-file -e "${HEAD_SHA}:${chart}/Chart.yaml" 2>/dev/null || continue
      printf '%s\n' "${chart}"
    done | sort -u
)

if [[ -z "${charts}" ]]; then
  echo "No Helm chart roots changed (only non-chart files under charts/) — skipping version bump check."
  exit 0
fi

rc=0
while IFS= read -r chart; do
  [[ -z "${chart}" ]] && continue
  if git diff --quiet "${BASE_SHA}" "${HEAD_SHA}" -- "${chart}"; then
    continue
  fi

  ver_base=$(chart_version_at "${BASE_SHA}" "${chart}")
  ver_head=$(chart_version_at "${HEAD_SHA}" "${chart}")

  if [[ -z "${ver_head}" ]]; then
    echo "::error title=Chart version check::${chart}: missing Chart.yaml or version field at HEAD"
    rc=1
    continue
  fi

  if [[ "${ver_base}" == "${ver_head}" ]]; then
    echo "::error title=Chart version check::${chart}: files changed but Chart.yaml version is still \"${ver_head}\". Bump version when changing the chart."
    rc=1
    continue
  fi

  echo "OK: ${chart} version ${ver_base:-<none>} -> ${ver_head}"
done <<< "${charts}"

exit "${rc}"
