#!/usr/bin/env bash
# Republish this repo's history to the "github" remote as a clean showcase
# mirror, stripping the private working-notes files from every commit.
# Optionally tag the published tip with a release version, which also
# pushes that tag to github (needed since `go install .../pmt@<version>`
# resolves tags against github specifically, not the primary origin):
#
#   scripts/publish-github.sh v0.2.0
#
# github is a read-only mirror, not a collaboration surface -- this
# rewrites history and force-pushes every time it runs. Never run this
# against a remote other people clone/fork/branch from. Because the
# rewrite changes commit hashes, a version tag must be (re)created here
# against the *rewritten* tip -- the same tag on origin's own history
# points at a different (unfiltered) commit object, which is expected.
set -euo pipefail

VERSION="${1:-}"
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

if [ -n "$VERSION" ]; then
  # Push the tag directly via refspec rather than creating a local tag
  # object first: this clone inherits whatever tags the source repo has
  # (including origin's own release tags), and filter-branch's
  # --tag-name-filter already rewrites any same-named one onto the new
  # commit -- creating another local tag of the same name would just
  # collide with that. A direct SHA:refspec push sidesteps it entirely.
  git push --force "$GITHUB_URL" "$(git rev-parse master):refs/tags/$VERSION"
  echo "Tagged and pushed $VERSION to the github remote."
fi

echo "Published clean history to the github remote."
