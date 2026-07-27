# Project Contract

## Project Overview
`pmtooling` is the Go source for `pmt`, a CLI tool that manages "issues" (bugs, features, etc.) in *other* git repositories using git branches and git worktrees, where each issue type is scaffolded from a template branch. This repo is only pmt's own source code — pmt is never run against this repo as if it were a target repo.

## Reference Files
- `doc/architecture.md` — repo separation model, worktree sibling convention, repo resolution/nested-worktree/bare-repo handling, config precedence
- `doc/commands.md` — command surface (v1 + Phase 7 v2 additions), exact git operations each command performs
- `doc/templates.md` — template ref-namespace convention, starter scaffold files, README front-matter schema, archive/reopen workflow
- `doc/edge-cases.md` — edge-case table, doubles as the implementation acceptance checklist
- `doc/external-repos.md` — recommended pattern for an issue worktree to reach a separate "code repo" (Claude Code `additionalDirectories` permission, not git submodules/nesting)
- `.claude/skills/pmt/SKILL.md` — the user-facing Claude Code skill teaching an agent how to *use* `pmt` (commands, workflows, gotchas); distinct from the `doc/` files above, which document how `pmt` is *built*

## User Preferences
- Ask first when something important is unclear or uncertain
- Establish a clear plan before starting substantive work

## Build And Test
- `go build ./...`
- `go test ./...` (all against real scratch git repos in `t.TempDir()` — never against this repo; see progress.md for the current test count)
- `go vet ./...`
- `go test -race ./...` requires cgo/a C compiler; not available in some dev environments (noted, not required)

## Safety Rails

### NEVER
- Run `pmt` against this (`pmtooling`) repo as a target repo — it only ever operates on separate target repos
- Use `go-git` for worktree operations — it has no linked-worktree support; shell out to the `git` CLI via `os/exec`
- Use `git switch --orphan` for template creation — it mutates the caller's current checkout/index; build orphan commits via plumbing (`hash-object`/`mktree`/`commit-tree`/`update-ref`) instead

### ALWAYS
- Keep template branches under `refs/heads/pmt/template/<type>`, never a bare `<type>` branch — it collides with issue branches `refs/heads/<type>/<title>` (git ref D/F conflict)
- Validate branch/title input with `git check-ref-format` (which already rejects trailing dots and embedded/trailing spaces on its own) plus a Windows reserved-device-name layer (`CON`/`NUL`/`COM1`/...) — see `doc/edge-cases.md`
- Read issue metadata via `git show <branch>:README.md`, not the filesystem — it must work with no worktree checked out

## Verification
- `go build ./...` and `go test ./...` must pass
- Edge-case integration tests run against scratch target repos created in a temp dir, never against `pmtooling` itself
- doc/edge-cases.md's table is an acceptance checklist — task_plan.md's Phase 6 section maps each row to its covering test(s)

## Compact Instructions
Preserve across compaction:
1. The ref-namespace decision (`pmt/template/<type>` vs bare `<type>`) and why — see `doc/templates.md`
2. Current phase of the phased implementation checklist (see `task_plan.md`/`progress.md` if present)
3. Any deviations from `doc/architecture.md` discovered during implementation, and why
