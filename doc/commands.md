# Commands

## v1 command surface

```
pmt new <type>[/<title>]  [--repo <path-or-nickname>]
pmt template new <name>   [--repo <path-or-nickname>]
pmt template list         [--repo <path-or-nickname>]
pmt list [--type <type>] [--json] [--repo <path-or-nickname>]
```

This is everything v1 shipped with. See `doc/architecture.md` for what was deliberately out of scope for v1 and the original v2 backlog note.

## v2 additions

### `pmt repo add/list/remove/set-default` (Phase 7a)

```
pmt repo add <nickname> <path> [--force]
pmt repo list
pmt repo remove <nickname>
pmt repo set-default <nickname>
```

Manages the user-level `repos:` nickname map (see doc/architecture.md#config) — previously hand-edit-only YAML.

- `add`: validates `<path>` resolves via `git.Discover` (a real git repo); errors on an existing nickname unless `--force`.
- `list`: prints `nickname -> path`, marking whichever one is `default_repo` with `(default)`.
- `remove`: errors on an unknown nickname; clears `default_repo` if it pointed at the removed nickname (and says so).
- `set-default`: errors on an unknown nickname.

These are entirely local, deterministic config edits — no new git mechanics.

### `PMT_DEFAULT_REPO` env var (Phase 8a)

A literal filesystem path, checked as a repo-selection fallback between cwd discovery and `default_repo` — see doc/architecture.md#repo-resolution for full precedence. Not a nickname, not managed by `pmt repo`; set it directly in your shell/session (e.g. `export PMT_DEFAULT_REPO=/home/james/work/clientA` or `$env:PMT_DEFAULT_REPO = "C:\work\clientA"`). If set but not a valid directory, `pmt` fails with a clear error rather than silently falling through to `default_repo`.

### `pmt close <type>/<title>` / `pmt reopen <type>/<title>` / `pmt list --archived` (Phase 7b)

```
pmt close <type>/<title>    [--repo <path-or-nickname>]
pmt reopen <type>/<title>   [--repo <path-or-nickname>]
pmt list --archived [--type <type>] [--json] [--repo <path-or-nickname>]
```

See doc/templates.md#archiving-issues-pmt-close--pmt-reopen for the full design and rationale (append-only archive, no manifest file). Summary of `pmt close`:

1. Error if the branch doesn't exist.
2. Determine the issue's worktree state via `git worktree list --porcelain`: registered-and-present, registered-but-prunable (directory manually deleted), or never registered (a hand-created branch). A present worktree with uncommitted changes (`git status --porcelain` non-empty) is refused, not force-cleaned.
3. Stamp `README.md` with `status: closed` + a `closed` RFC3339 timestamp — via the worktree if one is present on disk, otherwise via plumbing directly on the branch (mirroring how templates are created without a worktree).
4. Merge the stamped tip's tree into `refs/heads/pmt/archive` under `<type>/<title>/` (creating that branch on first use), via `ls-tree` + `mktree` tree surgery — replacing that one entry if it already existed (a prior close before a reopen), keeping every sibling issue's entry unchanged by SHA reference. Commit parents: `[stamped-tip, previous-archive-tip-if-any]`.
5. Remove the worktree if one was registered (`git worktree remove` handles both the present and prunable cases).
6. `git branch -D <type>/<title>`.

`pmt reopen <type>/<title>`:

1. Error if a live branch of that name already exists.
2. Walk `refs/heads/pmt/archive` backward via each commit's second parent, comparing the tree entry at `<type>/<title>` against the predecessor's, to find the *most recent* close (not necessarily the first) — error (`ErrNotArchived`) if never archived.
3. `git branch <type>/<title> <that commit's first parent>` — recreates the branch at its exact original tip, full history intact.
4. `git worktree add` a fresh worktree (after the same orphaned-directory check `pmt new` uses).
5. Stamp `README.md` back to `status: open`, clearing `closed`, as one more commit.

The archive entry itself is left untouched by reopen (see doc/templates.md) — it only disappears from `pmt list --archived` once the same issue is closed again.

### `pmt template new <name> --from <source>` / `pmt template update <name> --from <source>` (Phase 7d)

```
pmt template new <name> --from <path-or-nickname>     [--repo <path-or-nickname>]
pmt template update <name> --from <path-or-nickname>  [--repo <path-or-nickname>]
```

See doc/templates.md#sharing-templates-between-repos for the full design. `<source>` is resolved exactly like `--repo` (an existing path, or a nickname from the `repos:` map) but never falls back to cwd or `default_repo` — it always names a specific other repo explicitly.

`pmt template new <name> --from <source>`: a first-time import. Fetches `refs/heads/pmt/template/<name>` directly from `<source>` into the current repo (`git fetch <source-path> refs/heads/pmt/template/<name>:refs/heads/pmt/template/<name>`) — no merge involved, the local ref is created pointing at the exact same commit as the source. Errors if a template of that name already exists locally (same collision as plain `pmt template new`).

`pmt template update <name> --from <source>`: pulls in later changes to an already-imported template. Fetches into a scratch ref `refs/heads/pmt/template-incoming/<name>`, then:
- If local and incoming are the same commit, or local is already an ancestor of incoming (fast-forward possible) — updates `refs/heads/pmt/template/<name>` directly and deletes the scratch ref.
- If local is already ahead of (or equal to) incoming — nothing to do, deletes the scratch ref, reports "already up to date."
- If the two have diverged — **no automatic merge is attempted**. The scratch ref is left in place, and pmt prints instructions to merge manually with plain git (e.g. `git worktree add <path> pmt/template/<name>` then `git -C <path> merge pmt/template-incoming/<name>`). Errors with `ErrNotFound` if the template was never imported locally.

### `pmt get [<type>/<title>]` (Phase 9)

```
pmt get [<type>/<title>]  [--repo <path-or-nickname>]
```

Named "get" rather than "goto" because it never changes any directory itself — a subprocess cannot change its parent shell's working directory. It's deliberately a path-resolver: on success it prints exactly the issue's worktree path to stdout and nothing else, meant to be composed as `dir=$(pmt get <type>/<title>) && cd "$dir"` (the `&&` matters — it's what keeps a failed lookup from `cd`-ing to an unintended location on an empty/garbage string). Every other outcome writes a human-readable explanation to stderr via the normal cobra/main.go error path and exits non-zero, with stdout left empty:

1. Resolve repo. If `<type>/<title>` is omitted, read the branch checked out at the *actual invocation cwd* (`git symbolic-ref --short -q HEAD`, not the resolved repo root — these differ when cwd is already inside a linked worktree) and use it as the target if it's shaped like `<type>/<title>`; otherwise error with usage.
2. Look up the target among live issues (`issue.ListIssues`, scoped to its type for efficiency).
   - `WorktreeOK` → print the worktree path, done.
   - Any other worktree state (`prunable`/`orphaned`/`missing`) → error naming the state, points to `pmt list`.
3. Not live → look up among archived issues (`archive.ListArchived`, scoped to its type). Found → error explaining it's archived, with the exact `pmt reopen <type>/<title>` command.
4. Found nowhere → error, points to both `pmt list` and `pmt list --archived`.

### `pmt push [<type>/<title>] [--all] [--dry-run] [--remote <name>]` (distributed working, M2)

```
pmt push [<type>/<title>]  [--all] [--dry-run] [--remote origin]  [--repo <path-or-nickname>]
```

Publishes branches to the remote. Never forces. Never prints a remote URL: remotes carry embedded credentials, so output names branches and short commit ranges only, and git's own error text is scrubbed (`internal/git/scrub.go`, plus the exact configured URL is removed from any error).

- **What can be pushed:** live issue branches, `pmt/archive`, and `pmt/template/*`. The repo's default branch is never pushed by pmt; naming a non-managed branch is refused.
- **Selecting:** an explicit `<type>/<title>`; `--all` for every managed branch; or no argument to use the branch checked out in the current directory. An issue together with `--all` is an error.
- **Deciding:** each branch is compared with the remote's real tip read via `git ls-remote` (no fetch, no ref written), so `--dry-run` takes exactly the decisions a real run takes and changes nothing at all.

One line per branch, then a summary of counts:

| Verb | Meaning | Exit |
|---|---|---|
| `pushed` | fast-forwarded the remote branch (`abc1234..def5678`) | 0 |
| `new` | branch did not exist on the remote; created, upstream tracking set | 0 |
| `up-to-date` | nothing to push | 0 |
| `behind` | the remote is ahead of local; run `pmt sync` | 2 |
| `moved` | the remote has commits this repo has not fetched; run `pmt sync` | 2 |
| `diverged` | both sides have new commits; left alone, needs a person | 2 |
| `failed` | the push was rejected or errored (detail is URL-scrubbed); other branches still go ahead | 2 |

Exit status: `0` everything pushed or up to date; `2` finished but some branches need attention; `1` error. Pushing a new issue's branch promptly is also what stops a later sync from mistaking it for a closed issue.

### `pmt pull <type>/<title> [--dry-run] [--remote <name>]` (distributed working, M4)

Brings one issue from the remote onto this machine. It fetches, then:

- no local branch → create it (tracking the remote) and a worktree at the usual path;
- local branch but no worktree → fast-forward it if behind, and create the worktree;
- clean worktree, behind → `git merge --ff-only` in place;
- anything else is reported and left exactly as it is. Nothing is ever merged, reset or discarded.

Same verbs, URL scrubbing and exit codes as `push`, plus:

| Verb | Meaning | Exit |
|---|---|---|
| `created` | worktree (and local branch if needed) created | 0 |
| `updated` | clean worktree fast-forwarded (`abc1234..def5678`) | 0 |
| `up-to-date` | already current | 0 |
| `ahead` | local has unpushed commits; nothing to pull (`pmt push`) | 0 |
| `dirty` | worktree has uncommitted changes; untouched | 2 |
| `diverged` | both sides have new commits (`a...b`); untouched | 2 |
| `prunable` | worktree registered but its directory is gone; run `git worktree prune` | 2 |
| `orphaned` | a directory is where the worktree should go but git doesn't know it | 2 |

An issue that isn't on the remote is an error naming it, with near matches ("did you mean"); one that exists only locally points to `pmt push`. `pmt/*` branches are refused (`pmt sync` handles the archive and templates). With `--dry-run` nothing is fetched or written: the remote is read with `ls-remote`, and a remote tip this repo has never fetched is reported as `moved` rather than guessed.

### `pmt prune [--dry-run]` (distributed working, M5)

When an issue is closed with `pmt close` on one machine, its branch disappears from the remote but every other machine keeps a local copy. `pmt prune` deletes such a leftover only when all three hold:

1. the issue is in the archive (`pmt list --archived`) — pmt's own record of "closed", never git's `[gone]` upstream state;
2. no worktree (including a prunable one) has the branch checked out;
3. the tip holds nothing the archive lacks: every file in the tip's tree exists, byte-identical, in the archived copy of that issue. `README.md` may differ only by the close stamp (`status`, `closed`); any other README change counts. Files only the archive has are fine (the archive may hold later work).

| Verb | Meaning | Exit |
|---|---|---|
| `pruned` | deleted; detail prints the tip SHA and `git branch <name> <sha>` to restore it | 0 |
| `in-use` | closed elsewhere but still checked out here; kept, detail names the worktree | 2 |
| `differs` | tip has files the archive lacks or holds differently; kept, detail lists up to 5 paths | 2 |

Open issues and non-issue branches are never candidates. The deletion is guarded by the tip that was examined (`update-ref -d <ref> <sha>`), so a branch that moved meanwhile is not deleted. `--dry-run` reports the same verbs and deletes nothing. A `differs` branch needs a human: after checking the listed paths, delete it with plain `git branch -D` (the tip is shown).

## `pmt new <type>[/<title>]`

1. Split the argument on the first `/` into `type` and optional `title`. Resolve the target repo (`architecture.md`).
2. Validate `type` is a single valid ref segment: `git check-ref-format --branch <type>`.
3. Resolve the template ref `refs/heads/pmt/template/<type>`. Missing → error naming the type, points to `pmt template list` / `pmt template new`.
4. If `title` is omitted, auto-generate one (see below).
5. Validate the full branch name `<type>/<title>`:
   - pmt-level: `title` itself must not contain `/`.
   - `git check-ref-format --branch <type>/<title>` — catches `..`, `~^:?*[`, `@{`, leading/trailing/doubled `/`, `.lock` suffix, control characters, embedded spaces, and trailing dots (verified empirically: `check-ref-format` already rejects both `foo.` and `foo ` on its own).
   - Windows-safety layer: case-insensitive reserved device names (`CON, PRN, AUX, NUL, COM1-9, LPT1-9`) only — git has no opinion about these, so `check-ref-format` lets them through; trailing dot/space needed no separate check since `check-ref-format` already covers it.
6. Collision check on `refs/heads/<type>/<title>`:
   - Exists and the title was user-supplied → error.
   - Exists and the title was auto-generated → retry generation (increment, re-check) up to 5 times, then fail.
7. Check the computed worktree path isn't already occupied by an unrelated/orphaned directory. If it is, error and instruct manual cleanup (`git worktree prune` if stale) — pmt never deletes a directory it didn't just create.
8. `git branch <type>/<title> refs/heads/pmt/template/<type>` — branch created at the template's tip, no checkout yet.
9. `git worktree add <path> <type>/<title>` — this performs the checkout.
10. Stamp `README.md` front matter in the new worktree (title, branch, `created` in RFC3339 UTC, `status: open`, `template_ref` = the template's commit SHA at creation time), then commit inside that worktree:
    ```
    git -C <worktreePath> add README.md
    git -C <worktreePath> commit -m "pmt: initialize issue <type>/<title>"
    ```
11. Print the branch name and worktree path.

### Auto-title algorithm

- `git for-each-ref --format='%(refname:short)' refs/heads/<type>/`, strip the `<type>/` prefix.
- Keep only segments matching `^[0-9]+$` (a hand-created sibling like `bug/investigate-perf` is silently skipped, never misread as a number).
- `next = max(parsed) + 1` if any matched, else `1` — deliberately `max+1`, not `count+1`, so a deleted/renamed issue never causes a future number to be reused.
- Zero-padded to 4 digits (`%04d`; widens naturally past 9999).

## `pmt template new <name>` (scaffolding, no `--from`)

1. Resolve repo. Validate `name` has no `/` and `git check-ref-format --branch pmt/template/<name>` passes.
2. Collision check: `refs/heads/pmt/template/<name>` must not already exist → error naming the template.
3. Render starter files (`doc/templates.md`) with `<name>` substituted into the front-matter `type` placeholder.
4. Build the commit purely with plumbing (never touches the caller's working tree/index):
   ```
   git hash-object -w --stdin        # once per file: README.md, CLAUDE.md, .gitignore, .claude/settings.json
   git mktree                        # nested .claude tree
   git mktree                        # root tree
   git commit-tree <root-tree-sha> -m "pmt: initialize template '<name>'"   # orphan commit, no parent
   git update-ref refs/heads/pmt/template/<name> <commit-sha>
   ```
5. Print success and next steps (checkout via `git worktree add <path> pmt/template/<name>` or `git switch pmt/template/<name>`, edit, commit).

## `pmt template list`

`git for-each-ref --format='%(refname:short)' 'refs/heads/pmt/template/*'`, strip the `pmt/template/` prefix, print sorted. Empty result → hint to run `pmt template new <name>`.

## `pmt list [--type <type>] [--json]`

1. Resolve repo.
2. `git for-each-ref --format='%(refname:short)%09%(objectname)%09%(committerdate:iso-strict)' refs/heads/`.
3. Filter to branches whose first `/`-segment has a matching `pmt/template/<segment>` ref — this is what excludes unrelated branches (e.g. `release/1.0`) from ever appearing as a false issue. Apply `--type` filter if given.
4. `git worktree list --porcelain` → map `branch → {path, prunable}` (`prunable` is git's own signal that a registered worktree's directory is missing on disk).
5. For branches with no worktree entry at all, check whether a directory exists at the expected sibling path anyway — report as `orphaned dir` if so (leftover directory, no worktree registration; mirror image of the `prunable` case).
6. Read metadata via `git show <branch>:README.md` (works with no worktree checked out). Parse front matter; a parse failure is **non-fatal** — the row still renders with `branch`/worktree-status columns, metadata column shows `<unparseable>`.
7. Render as a table: `BRANCH | STATUS | CREATED | WORKTREE`. `--json` emits the same rows via `encoding/json`.
