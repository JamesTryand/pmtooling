# Architecture

## Repo separation

`pmtooling` (this repo) contains only pmt's own Go source. All issue/template branches and worktrees that pmt creates live in a **target repo** — a separate git repository (e.g. a client's project) that the user points `pmt` at. pmt is never run against `pmtooling` itself as a target.

## Core model

- An **issue** is a git branch in the target repo, named `<type>/<title>` (e.g. `bug/dboverflow`).
- A **template** is a git branch in the target repo, named `pmt/template/<type>` (e.g. `pmt/template/bug`). See `templates.md` for why this namespace is used instead of a bare `<type>` branch.
- Each issue gets its own **worktree**, checked out to a sibling directory next to the target repo's main checkout.

## Worktree sibling convention

```
mainRepoRoot        = C:\work\clientA
worktreesRoot        = C:\work\clientA.worktrees        (sibling of mainRepoRoot: "<basename>.worktrees")
issue worktree path  = worktreesRoot\<type>\<title>      e.g. C:\work\clientA.worktrees\bug\dboverflow
```

`worktreesRoot` can be overridden per target repo via repo-local `.pmt.yaml` (`worktrees_dir: <path>`) — needed if the repo's basename isn't filesystem-safe as a sibling name, or worktrees should live on a different volume.

For a **bare** repo (Phase 7c), `mainRepoRoot` is the bare repo's own path (see Repo resolution below), and `<basename>` has a trailing `.git` stripped first if present — a bare repo conventionally named `clientA.git` gets sibling worktrees at `clientA.worktrees`, not `clientA.git.worktrees`. Purely a naming nicety, not a correctness requirement.

## Repo resolution

Precedence, evaluated in this order:
1. `--repo <value>` flag: if `value` is an existing directory, use it directly; otherwise treat it as a nickname and look it up in the user-level config's `repos:` map. Unknown nickname is a hard error listing known nicknames.
2. No `--repo`: discover from the current working directory.

Discovery is one call:
```
git rev-parse --path-format=absolute --git-dir --git-common-dir --is-bare-repository
```
(`--show-toplevel` is deliberately omitted: it hard-fails with "fatal: this operation must be run in a work tree" inside a bare repo, which would break bare-repo detection in the very call meant to catch it. Everything pmt needs — the main root, and linked-worktree detection — comes from `--git-dir`/`--git-common-dir` alone.)
- Non-zero exit (not inside a git repo) and no `--repo` given → error: pass `--repo <path>` or `--repo <nickname>`.
- **Bare repos are supported (Phase 7c)**: for a bare repo, `git-dir == git-common-dir == the bare repo's own path` (verified empirically — not a `.git` subdirectory), so the main repo root is the bare repo's path itself, not its parent. `IsBare` is deliberately defined as "is the *main* repo (git-common-dir) bare," not "is the current invocation location bare" — those differ when invoked from inside a linked worktree of a bare repo (a linked worktree always has a real working tree, so raw `--is-bare-repository` reports `false` there even though the repo it belongs to is bare; re-checked directly against `git-common-dir` in that case, which correctly distinguishes a bare repo's own path from a normal repo's `.git` directory).
- If `git-dir` and `git-common-dir` differ, the cwd is inside a **linked worktree** — including the case of running `pmt` from inside one of its own issue worktrees. In every case, the canonical main repo root (`MainRoot()`) is derived from `git-common-dir` — the bare repo's own path if bare, otherwise `filepath.Dir(git-common-dir)` — and pmt always operates relative to that derived root, not the worktree it happened to be invoked from.

## Config

- **User-level** (`os.UserConfigDir()`, e.g. `%APPDATA%\pmt\config.yaml` on Windows; overridable via the `PMT_CONFIG_HOME` env var, analogous to `XDG_CONFIG_HOME` — mainly so tests never touch a real developer's config, but also usable to point pmt at an isolated config elsewhere):
  ```yaml
  repos:
    clientA: C:\work\clientA
    clientB: /home/james/work/clientB
  default_repo: clientA   # optional
  ```
  Managed via `pmt repo add/list/remove/set-default` (§ below) — no longer hand-edit-only as of Phase 7a.
- **Repo-local** `.pmt.yaml` at the target repo's root (intended to be committed to the target repo, like `.editorconfig`):
  ```yaml
  worktrees_dir: ../clientA.worktrees   # optional override
  title_pad_width: 4                    # optional override
  ```
  For a bare repo this is just `<bare-repo-path>/.pmt.yaml` — no special-casing needed; a bare repo is a plain directory (containing `HEAD`, `objects/`, `refs/`, ...), and an extra non-git file living alongside those is perfectly ordinary.
- **Repo selection precedence** (which target repo to use): `--repo` flag (path or nickname) > cwd discovery > user-level `default_repo` (nickname, used as a fallback only when cwd isn't inside any git repo and no `--repo` was given).
- **Config precedence** (once a repo is selected): the resolved repo's `.pmt.yaml`, if present, overrides pmt's built-in defaults (sibling worktree convention, `title_pad_width: 4`). There is no further chain — repo-local config always wins over built-in defaults, and user-level config never supplies `worktrees_dir`/`title_pad_width`.
- The `git-dir`/`git-common-dir` → `MainRoot()` derivation (including bare-repo handling) applies uniformly after resolution, regardless of whether the repo came from `--repo`, cwd discovery, or the `default_repo` fallback.

## Issue metadata — no central manifest

pmt deliberately has no database or manifest file tracking issue state. Metadata (type, title, branch, status, created date, template ref) lives as YAML front matter inside each issue's own `README.md`. This means:
- It's grep-able with plain text tools across worktrees.
- It's readable via `git show <branch>:README.md` even when no worktree is checked out for that branch — this is what makes `pmt list` work without any central state.

See `templates.md` for the exact front-matter schema.

## Non-goals

- No push, no PR/issue creation, no GitHub/GitLab API integration — purely local git (still true after the Phase 7 v2 work; not selected for implementation).
- ~~No `pmt close`/cleanup command~~ — implemented as Phase 7b (`pmt close`/`pmt reopen`, see doc/templates.md).
- ~~No config-editing subcommands~~ — implemented as Phase 7a (`pmt repo add/list/remove/set-default`).
- ~~No bare-repo support~~ — implemented as Phase 7c (this doc's Repo resolution and Worktree sibling convention sections above).
- No release pipeline — distributed via `go install` only.
