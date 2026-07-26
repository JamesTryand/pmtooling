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
---

# <title>

Describe the issue here.
```

| field | set by | meaning |
|---|---|---|
| `type` | template author (fixed per template) | issue type / template name |
| `title` | `pmt new` | the title segment, user-given or auto-generated |
| `branch` | `pmt new` | full `<type>/<title>`, for self-describing greppability without parsing git |
| `status` | `pmt new` (always `open` in v1) | lifecycle state — only `open` is ever written in v1; field exists for forward compatibility with a future `pmt close` |
| `created` | `pmt new` | RFC3339 UTC timestamp |
| `template_ref` | `pmt new` | commit SHA of the template branch at the moment the issue was created, for traceability |

`pmt new` parses the template's existing front matter (inserting a fresh block if absent), fills in the issue-specific fields, writes the file, and commits it inside the new issue's own worktree. This commit is what lets `pmt list` read metadata via `git show <branch>:README.md` without needing any worktree to exist.
