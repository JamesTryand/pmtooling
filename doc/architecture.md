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

## Repo resolution

Precedence, evaluated in this order:
1. `--repo <value>` flag: if `value` is an existing directory, use it directly; otherwise treat it as a nickname and look it up in the user-level config's `repos:` map. Unknown nickname is a hard error listing known nicknames.
2. No `--repo`: discover from the current working directory.

Discovery is one call:
```
git rev-parse --path-format=absolute --show-toplevel --git-dir --git-common-dir --is-bare-repository
```
- Non-zero exit (not inside a git repo) and no `--repo` given → error: pass `--repo <path>` or `--repo <nickname>`.
- `--is-bare-repository` true → explicit v1 error; the sibling-worktree convention has no defined location for a bare repo.
- If `git-dir` and `git-common-dir` differ, the cwd is inside a **linked worktree** — including the case of running `pmt` from inside one of its own issue worktrees. In every case, the canonical main repo root is derived as `filepath.Dir(git-common-dir)`, and pmt always operates relative to that derived root, not the worktree it happened to be invoked from.

## Config

- **User-level** (`os.UserConfigDir()`, e.g. `%APPDATA%\pmt\config.yaml` on Windows):
  ```yaml
  repos:
    clientA: C:\work\clientA
    clientB: /home/james/work/clientB
  default_repo: clientA   # optional
  ```
- **Repo-local** `.pmt.yaml` at the target repo's root (intended to be committed to the target repo, like `.editorconfig`):
  ```yaml
  worktrees_dir: ../clientA.worktrees   # optional override
  title_pad_width: 4                    # optional override
  ```
- Precedence: `--repo` flag > repo-local `.pmt.yaml` > cwd discovery > user-level `default_repo`.

## Issue metadata — no central manifest

pmt deliberately has no database or manifest file tracking issue state. Metadata (type, title, branch, status, created date, template ref) lives as YAML front matter inside each issue's own `README.md`. This means:
- It's grep-able with plain text tools across worktrees.
- It's readable via `git show <branch>:README.md` even when no worktree is checked out for that branch — this is what makes `pmt list` work without any central state.

See `templates.md` for the exact front-matter schema.

## Non-goals (v1)

- No push, no PR/issue creation, no GitHub/GitLab API integration — purely local git.
- No `pmt close`/cleanup command — issue lifecycle beyond creation is out of scope for v1.
- No config-editing subcommands — user/repo-local config files are hand-edited YAML.
- No release pipeline — distributed via `go install` only.
