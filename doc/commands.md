# Commands (v1)

```
pmt new <type>[/<title>]  [--repo <path-or-nickname>]
pmt template new <name>   [--repo <path-or-nickname>]
pmt template list         [--repo <path-or-nickname>]
pmt list [--type <type>] [--json] [--repo <path-or-nickname>]
```

Nothing else ships in v1. See `doc/architecture.md` for non-goals and the v2 backlog.

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

## `pmt template new <name>`

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
