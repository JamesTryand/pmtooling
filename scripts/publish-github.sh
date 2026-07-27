#!/usr/bin/env bash
# Republish this repo's history to the "github" remote as a clean showcase
# mirror, stripping the private working-notes files from every commit.
#
# github is a read-only mirror, not a collaboration surface -- this
# rewrites history and force-pushes every time it runs. Never run this
# against a remote other people clone/fork/branch from.
set -euo pipefail

PRIVATE_PATHS=(task_plan.md progress.md findings.md)
REPO_ROOT="$(git rev-parse --show-toplevel)"
GITHUB_URL="$(git -C "$REPO_ROOT" remote get-url github)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

git clone --no-local "$REPO_ROOT" "$WORKDIR"
cd "$WORKDIR"

FILTER_BRANCH_SQUELCH_WARNING=1 git filter-branch --force --index-filter \
  "git rm --cached --ignore-unmatch ${PRIVATE_PATHS[*]}" \
  --prune-empty --tag-name-filter cat -- --all

git push --force "$GITHUB_URL" master

echo "Published clean history to the github remote."
