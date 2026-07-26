# Templates

## Ref namespace: `pmt/template/<type>`, not bare `<type>`

Git stores branch refs hierarchically under `refs/heads/...`. A branch named `bug` and a branch named `bug/dboverflow` cannot coexist: the moment one exists, creating the other fails, because `refs/heads/bug` (a ref) and `refs/heads/bug/dboverflow` (which requires `bug` to be a path segment) collide. Since issue branches are the user-facing name `<type>/<title>` (e.g. `bug/dboverflow`, per `pmt new bug/dboverflow`), template branches cannot also be bare `<type>` branches.

**Resolution**: template branches live at `refs/heads/pmt/template/<type>` (e.g. `pmt/template/bug`). This is isolated to one lookup (`internal/template.RefFor(type) string` in the eventual implementation) so it's the single place the convention is expressed. Issue-facing naming is completely unaffected — `pmt new bug/dboverflow` still creates and checks out branch/worktree `bug/dboverflow`; only the template's own branch name differs from the bare `bug` a user might otherwise expect.

A pre-existing, unrelated branch literally named `bug` (e.g. from an old convention or hand-created) is simply a different, unrelated ref under this scheme — pmt only ever reads/writes `pmt/template/<type>`.

## Template lifecycle

Templates are plain git branches. `pmt template new <name>` is a convenience scaffolder — a user can equally hand-create `pmt/template/<name>` with plain git commands and it will work identically. pmt does not manage template *content* beyond initial scaffolding: after `pmt template new`, the user checks it out, edits, and commits like any other branch.

`pmt template new <name>` builds the initial commit purely with git plumbing (`hash-object`, `mktree`, `commit-tree`, `update-ref`) rather than `git switch --orphan`, so it never touches the caller's current checkout or index in the main repo.

## Starter scaffold files

Every new template branch is created with:

- **`README.md`** — carries the front-matter schema below, with only `type` filled in.
- **`CLAUDE.md`** — generic per-issue-worktree agent instructions; template authors are expected to customize this per issue type (e.g. a `bug` template emphasizing reproduce → failing test → fix → verify).
- **`.gitignore`** — a minimal, language-agnostic stub; template authors should replace it with something appropriate to the target repo's stack.
- **`.claude/settings.json`** — ships as `{}`. pmt's only contract is copying this file verbatim from the template into every issue worktree created from it; permission/allow-list content is entirely the template author's responsibility.

## README front-matter schema

Every issue's `README.md` carries:

```markdown
---
pmt:
  type: bug
  title: ""
  branch: ""
  status: open
  created: ""
  template_ref: ""
  closed: ""
---

# <title>

Describe the issue here.
```

| field | set by | meaning |
|---|---|---|
| `type` | template author (fixed per template) | issue type / template name |
| `title` | `pmt new` | the title segment, user-given or auto-generated |
| `branch` | `pmt new` | full `<type>/<title>`, for self-describing greppability without parsing git |
| `status` | `pmt new` (`open`), `pmt close` (`closed`), `pmt reopen` (`open` again) | lifecycle state |
| `created` | `pmt new` | RFC3339 UTC timestamp |
| `template_ref` | `pmt new` | commit SHA of the template branch at the moment the issue was created, for traceability |
| `closed` | `pmt close` (RFC3339 UTC timestamp), cleared by `pmt reopen` | when the issue was last closed; `omitempty` — absent entirely until first closed |

`pmt new` parses the template's existing front matter (inserting a fresh block if absent), fills in the issue-specific fields, writes the file, and commits it inside the new issue's own worktree. This commit is what lets `pmt list` read metadata via `git show <branch>:README.md` without needing any worktree to exist.

## Archiving issues (`pmt close` / `pmt reopen`)

`pmt close <type>/<title>` doesn't delete an issue's history — it merges the issue's final tree into a dedicated `refs/heads/pmt/archive` branch (auto-created on first use) under the path `<type>/<title>/`, then removes the worktree (if any) and deletes the issue branch. The archive commit's parents are `[issue-tip, previous-archive-tip-if-any]` — this is what keeps the closed issue's commits reachable (and therefore safe from `git gc`) even after the branch itself is deleted, and lets `pmt reopen <type>/<title>` recreate the branch at its **exact original tip**, full history intact, by reading that commit's first parent. No separate manifest file is needed; this is pure git structure, matching the project's existing "grep-able files in git" philosophy one level deeper.

**The archive is append-only in spirit**: `pmt reopen` never removes anything from `refs/heads/pmt/archive` — it only recreates the live branch. If that issue is closed again later, the *same* `<type>/<title>` path is updated in place (via `ls-tree`+`mktree` tree surgery — replacing just that one entry, keeping every sibling issue's entry unchanged by SHA reference) with the new content, while the full commit history — including everything from before the reopen — remains walkable through ordinary commit ancestry, since the recreated branch's later commits naturally have the original pre-close tip as an ancestor. Until an issue is closed again, `pmt list --archived` keeps showing its last-archived (now potentially stale) snapshot — this is intentional: the archive reflects "what this looked like when it was last closed," not live state.

## Sharing templates between repos

Since every target repo is just a local git repository (per `repos:` in the user config), a template can be shared between repos with nothing more than `git fetch` against another repo's path — no registry, no export/import file format.

`pmt template new <name> --from <source>` (first-time import): fetches `refs/heads/pmt/template/<name>` directly from `<source>` into the current repo's identical ref. The two repos now share a **real common ancestor** — the exact commit that was fetched — which is what makes a later `pmt template update` a genuine, ordinary 3-way-mergeable relationship rather than two unrelated histories (verified empirically: fetching a specific ref between two local repos this way, then having each side commit independently, produces a normal mergeable divergence, including correct conflict detection, not "refusing to merge unrelated histories").

`pmt template update <name> --from <source>` (pulling in later changes): fetches into a scratch ref `refs/heads/pmt/template-incoming/<name>` and inspects the relationship between it and the local `refs/heads/pmt/template/<name>`:
- Same commit, or local is an ancestor of incoming → fast-forward the local ref, delete the scratch ref.
- Incoming is an ancestor of (or equal to) local → nothing to do, delete the scratch ref, report up to date.
- Diverged → **no automatic merge**. The scratch ref is left in place for the user to inspect and merge manually with plain git. `git merge-tree --write-tree` could do a merge without needing a checkout at all, but it requires git 2.38 — above this project's declared floor of 2.31 — and raising the floor solely for this one feature wasn't judged worthwhile. This also matches the template lifecycle's existing stance: pmt scaffolds and shares templates, but never manages their ongoing *content*.

Both directions reuse the exact same source resolution as `--repo` (an existing path, or a nickname from the user config's `repos:` map) — there is no separate addressing mechanism for template sharing.
