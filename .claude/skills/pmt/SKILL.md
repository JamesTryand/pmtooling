---
name: pmt
description: 'Manage project issues as git branches and worktrees using the `pmt` CLI -- create, list, close, and reopen issues, and create/list/import/update issue templates. Use whenever the user wants to start work on a bug/feature/chore in a repo that uses pmt, list open or archived issues, close out or reopen an issue, or set up/share issue templates. Trigger phrases: "start a new bug/feature/task", "create an issue for X", "what issues are open", "close this issue", "reopen issue X", "set up a new issue template", "import the bug template from <repo>".'
user-invocable: true
allowed-tools: "Bash Read Write Edit Glob Grep"
metadata:
  version: "1.0.0"
---

# pmt: git-native issue management

`pmt` manages project issues as git branches and worktrees in a **target repo** — any git repository the user points it at (often the actual codebase, but not necessarily — see "External code repos" below). `pmt` is a separate CLI binary; this skill teaches you how to *use* it, not how it's built.

## Core model

- **Issue** = a git branch named `<type>/<title>` (e.g. `bug/dboverflow`), checked out into its own worktree next to the target repo's main checkout (e.g. `../<repo>.worktrees/bug/dboverflow`).
- **Template** = a git branch `pmt/template/<type>` that a new issue is scaffolded from. Its `README.md`, `CLAUDE.md`, `.gitignore`, `.claude/settings.json` — and anything else the template author has since added (more permissions, MCP configs, skills) — become that issue's starting point.
- **Archive**: closing an issue (`pmt close`) doesn't delete its history — it merges the issue into a `pmt/archive` branch, then removes the live branch and worktree. `pmt reopen` restores it later with full history intact, as if it had never been closed.

## Before doing anything: confirm the target repo

pmt resolves which repo to operate on in this order: `--repo <path-or-nickname>` flag > current working directory (if it's inside a git repo) > `PMT_DEFAULT_REPO` env var (a path) > the user config's `default_repo` nickname. If it's unclear which repo the user means, run `pmt repo list` to see configured nicknames, or just ask — don't guess when it matters.

Confirm `pmt` is installed and on PATH first (`pmt version`). If it's not found, tell the user to run:
```
go install github.com/JamesTryand/pmtooling/cmd/pmt@latest
```

## Command reference

| Command | What it does |
|---|---|
| `pmt new <type>[/<title>]` | Create a new issue. Omit `<title>` to auto-generate one (`bug/0001`, `bug/0002`, ...). Prints the branch name and worktree path. |
| `pmt list [--type <t>] [--json]` | List live (open) issues: branch, status, created date, worktree state/path. |
| `pmt list --archived [--type <t>] [--json]` | List closed issues instead of live ones. |
| `pmt close <type>/<title>` | Archive and remove a finished issue. **Refuses if the worktree has uncommitted changes.** |
| `pmt reopen <type>/<title>` | Restore a previously closed issue — full history intact — as a live branch + worktree again. |
| `pmt get [<type>/<title>]` | Print an issue's worktree path (nothing else on success) so you can `cd` there. Omit the argument to resolve the *current* branch instead. Not found/archived/no-worktree cases explain why on stderr and exit non-zero, printing nothing on stdout. |
| `pmt template new <name>` | Scaffold a brand-new template branch. |
| `pmt template new <name> --from <source>` | Import a template from another repo instead of scaffolding a blank one. |
| `pmt template list` | List available template types in the target repo. |
| `pmt template update <name> --from <source>` | Pull in changes to an already-imported template. Fast-forwards automatically when safe; if diverged, does **not** auto-merge — leaves the incoming commit at `pmt/template-incoming/<name>` for a manual merge. |
| `pmt repo add/list/remove/set-default` | Manage the user-level nickname → path map used by `--repo`. |

`--repo <path-or-nickname>` works on every command above and overrides cwd-based resolution.

## Common workflows

**Starting a new piece of work**
1. `pmt new <type>/<title>` (or `pmt new <type>` for an auto-generated title).
2. `cd` into the printed worktree path.
3. **Read that worktree's own `CLAUDE.md` and `README.md` before doing anything else.** The template author may have left type-specific instructions there (how to reproduce a bug, what "done" means, links to further context), and the README's front matter carries the issue's own metadata (`type`, `title`, `branch`, `status`, `created`).
4. If the work is driven by an external ticket (an Azure DevOps work item, a GitHub issue/PR, or just a paragraph someone gave you) rather than a clear spec, don't start coding from a vague description — paste the raw ticket content and ask to run the `better-init` skill first. It reuses that content as the project intro instead of re-asking for it, and drafts a scoped plan for the issue (confirmed with the user before anything's written). See the project's own `README.md` "Getting started" section for a worked example prompt.
5. Do the work, committing as you go, inside that worktree.

**Checking on existing work**
- `pmt list --json` if you need to reason about several issues programmatically; plain `pmt list` for a quick human-readable check.
- The `WORKTREE` column may show `(prunable)` (the worktree directory was deleted outside pmt) or `(orphaned dir)` (a stray directory sits at the expected path but isn't registered) — these are informational; don't act on them (e.g. delete anything) without the user's OK.

**Finishing up**
- Commit everything in the issue's worktree, *then* `pmt close <type>/<title>`. If it's refused, there are uncommitted changes — check `git status` in that worktree first.
- If something was closed too early or needs more work, `pmt reopen <type>/<title>` brings it back exactly as it was.

**Jumping back into an existing issue**
- `pmt get` never changes your directory itself — a subprocess can't change its parent shell's cwd, and `pmt` is no exception (that's why it's called "get," not "goto"). It only ever prints a path (or, on failure, nothing to stdout and an explanation on stderr). Use it as `dir=$(pmt get <type>/<title>) && cd "$dir"` — the `&&` matters, since it's what stops a failed lookup from `cd`-ing anywhere on empty output.
- Bare `pmt get` (no argument) resolves the branch currently checked out at cwd instead of taking one as an argument — handy for "what issue am I even in right now," or for normalizing back to a worktree's root from a subdirectory of it.
- If the issue isn't live, the error tells you why: archived (with the exact `pmt reopen` command to fix it) or not found anywhere (with both `pmt list` and `pmt list --archived`) — read it before trying again rather than guessing.

**Setting up templates**
- `pmt template new <name>` gives a blank starting point; check it out (`git worktree add <path> pmt/template/<name>`), edit its files, commit.
- To reuse an existing template from another project instead of writing one from scratch: `pmt template new <name> --from <path-or-nickname>`, and `pmt template update <name> --from <source>` later to pull in improvements.

## External code repos

pmt's target repo tracks *issues*; the actual codebase an issue is about sometimes lives in a separate "code repo" entirely. That's normal — don't assume the issue's own worktree contains the full project. Look for permission grants in the worktree's `.claude/settings.json` (a `permissions.additionalDirectories` entry) pointing at the real code repo, or ask the user where the code lives if it isn't obvious.

## Gotchas

- Never run `pmt` against pmt's own source repo (`pmtooling`) as a target — it's built to manage *other* projects' issues, not its own.
- `pmt close` never force-cleans a dirty worktree — it errors instead. Don't work around this by discarding changes without the user's explicit OK.
- Auto-generated titles (`bug/0001`, `bug/0002`, ...) are scoped per type and never reused, even after an issue is closed or deleted — a gap in the numbering isn't a sign anything's wrong.
- Templates can carry more than the 4 starter files — MCP configs, `.claude/skills/`, permission grants for external repos, anything a template author has committed. Whatever's on the template branch at the moment `pmt new` runs is exactly what the new issue inherits.

## Going deeper

- [building-templates.md](building-templates.md) — how to shape a template's `CLAUDE.md`, `.claude/settings.json` permissions, `.mcp.json`, and project-scoped skills for the kind of issue it produces, plus the `.claude/`-gitignore footgun to avoid.
- [workflow-patterns.md](workflow-patterns.md) — turning a freshly created issue's placeholder README into a concrete plan, and when to reach for `/loop` or a plan-file-driven iteration instead of a single pass.
