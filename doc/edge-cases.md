# Edge cases

Explicit v1 behavior for every edge case identified during design. This table doubles as an acceptance checklist — each row should have a corresponding integration test, run against a scratch target repo (never against `pmtooling` itself).

| Edge case | Resolution |
|---|---|
| `pmt new bug` (no title) | Auto-generate a `bug/0001`-style title (see `commands.md`) |
| `<type>` template doesn't exist | Error naming the type, points to `pmt template list` / `pmt template new` |
| `<type>/<title>` branch already exists | Error if the title was user-supplied; auto-retry up to 5x then error if the title was auto-generated |
| Invalid/unsafe characters in `<title>` | `git check-ref-format` (authoritative for git ref rules — this alone already rejects trailing dots and embedded/trailing spaces) plus a Windows reserved-device-name layer (`CON`, `NUL`, `COM1`, ...), which git has no native opinion about |
| Worktree directory already exists at the target path, but the branch doesn't | Error, no auto-delete; instructs manual cleanup (and `git worktree prune` if it was a stale worktree) |
| Branch exists but its worktree was manually deleted | Detected via `git worktree list --porcelain`'s `prunable` flag, surfaced in `pmt list` |
| Running `pmt` outside any git repo, no `--repo`/config given | Error: pass `--repo <path>` or `--repo <nickname>` |
| Running `pmt` from inside one of its own issue worktrees | `git-dir` vs `git-common-dir` divergence detected; operations always resolve to the derived main repo root regardless of invocation location |
| `pmt template new <name>` when that template already exists | Error naming the template |
| Listing issues with no central manifest | `git for-each-ref` cross-referenced with `git worktree list --porcelain`; metadata read via `git show <branch>:README.md` |
| Hand-edited/corrupted README front matter | Non-fatal parse failure; the row still lists, metadata column shows `<unparseable>` |
| Target repo is bare | Explicit v1 error — the sibling-worktree convention has no defined location for a bare repo |

## Phase 7b edge cases (`pmt close` / `pmt reopen`)

| Edge case | Resolution |
|---|---|
| `pmt close` on a branch that doesn't exist | Error naming the branch |
| `pmt close` with uncommitted changes in the worktree | Refused (`ErrDirtyWorktree`), not force-cleaned; nothing is stamped, archived, removed, or deleted |
| `pmt close` when the worktree is registered but its directory was manually deleted (prunable) | Stamped via plumbing directly on the branch (no directory to write into); `git worktree remove` still cleans up the stale registration so the branch can be deleted |
| `pmt close` on a hand-created branch that never had a worktree | Stamped via plumbing; worktree removal step is skipped entirely (nothing was ever registered) |
| `pmt reopen` on a name with no archived entry | Error (`ErrNotArchived`) |
| `pmt reopen` when a live branch of that name already exists | Error — refuses to recreate over a live branch |
| `pmt reopen` after the same issue was previously closed, reopened, and closed again | Finds the *most recent* close (tree-content comparison while walking the archive's second-parent chain), not the stale first one — this is the exact scenario an early parent-position-based design got wrong; see task_plan.md's Decisions Made |
| Two different issues both ever archived | Fully independent — closing/reopening one never touches the other's archive entry (verified: `ls-tree`+`mktree` tree surgery replaces only the target path, keeps every sibling by SHA reference) |

## Deliberately out of scope for v1

- Push, PR/issue API integration (GitHub/GitLab) — purely local git for now, and still deferred as of the Phase 7 v2 work (not selected for implementation).
- ~~`pmt close` / issue cleanup~~ — implemented as Phase 7b (`pmt close`/`pmt reopen` with an append-only archive workflow, see doc/templates.md and doc/commands.md).
- ~~Config-editing subcommands (`pmt repo add/list/remove`)~~ — implemented as Phase 7a: `pmt repo add/list/remove/set-default`, see doc/commands.md.
- Release pipeline — `go install` only.
