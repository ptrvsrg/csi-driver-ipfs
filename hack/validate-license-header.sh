#!/bin/bash

# Copyright 2026 ptrvsrg.
#
# Licensed under the Apache License, Version 2.0 (the "License");
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

set -euo pipefail

find . -type d \( -name "deploy" -o -name "charts" -o -name "node_modules" -o -name "vendor" \) -prune -o -type f \( -name "*.go" -o -name "*.sh" -o -name "*.yaml" -o -name "*.yml" \) -print | while read -r file; do
  case "$file" in
  **/mock_*.go)
    continue
    ;;
  **/volume_limits_table.go)
    continue
    ;;
  *.go)
    comment_prefix="//"
    ;;
  *.sh | *.yaml | *.yml)
    # shellcheck disable=SC2034
    comment_prefix="#"
    ;;
  esac

  if ! grep -q "Copyright" "$file"; then
    printf "Check license header %s - FAIL\n" "$file"
    exit 1
  fi
  printf "Check license header %s - OK\n" "$file"
done
