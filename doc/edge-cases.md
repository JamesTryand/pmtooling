# Edge cases

Explicit behavior for every edge case identified during design (v1 and the Phase 7 v2 work). This table doubles as an acceptance checklist — each row should have a corresponding integration test, run against a scratch target repo (never against `pmtooling` itself).

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
| Target repo is bare | Supported as of Phase 7c — see the Phase 7c row below; no longer rejected |

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

## Phase 7c edge cases (bare-repo support)

| Edge case | Resolution |
|---|---|
| Bare repo as the target repo | Supported — `MainRoot()` resolves to the bare repo's own path (not its parent); `pmt new`/`pmt template new`/`pmt list`/`pmt close`/`pmt reopen` all work unchanged since they're already plumbing-based and never required a working tree in the *main* repo itself (only linked worktrees have one) |
| Invoked from inside a linked worktree of a bare repo | `IsBare` correctly reflects the *main* repo's bareness, not the worktree's own (which is never bare) — verified empirically that raw `git rev-parse --is-bare-repository` reports `false` here, so bareness is re-checked directly against `git-common-dir` |
| Bare repo conventionally named `<name>.git` | Sibling worktrees land at `<name>.worktrees`, not `<name>.git.worktrees` — a trailing `.git` is stripped from the basename first (naming nicety, not correctness) |
| Repo-local `.pmt.yaml` for a bare repo | No special-casing — it's just `<bare-repo-path>/.pmt.yaml`, an ordinary extra file alongside the bare repo's own `HEAD`/`objects`/`refs` |

## Phase 7d edge cases (template sharing between repos)

| Edge case | Resolution |
|---|---|
| `pmt template new <name> --from <source>` when the source doesn't have that template | Error — `git fetch` fails clearly ("couldn't find remote ref") |
| `pmt template new <name> --from <source>` when a template of that name already exists locally | Error (`ErrExists`) — same collision as scaffolding a new one; this is a first-import command, not an update |
| `pmt template update <name> --from <source>` when the template was never imported locally | Error (`ErrNotFound`), points to `pmt template new <name> --from <source>` |
| `pmt template update` when nothing has changed upstream | Reports "already up to date," scratch ref cleaned up, nothing else touched |
| `pmt template update` when the local copy is already ahead of the source | Same as above — treated as up to date, not an error |
| `pmt template update` when local and source have diverged | No automatic merge; local template is completely untouched, the fetched commit is left at an inspectable `pmt/template-incoming/<name>` ref with manual-merge instructions printed |
| `--from` value is a bad revision / invalid nickname | `merge-base --is-ancestor` distinguishes a genuine error (e.g. invalid revision, exit 128) from a valid "not an ancestor" answer (exit 1) — never silently misreported as "not an ancestor" |

## Phase 8a edge cases (`PMT_DEFAULT_REPO` env var)

| Edge case | Resolution |
|---|---|
| `PMT_DEFAULT_REPO` set to a path that doesn't exist or isn't a directory | Hard error naming the env var and value — never silently falls through to `default_repo` |
| `PMT_DEFAULT_REPO` set, but cwd is already inside a git repo | Ignored — cwd discovery still wins per the precedence order |
| `PMT_DEFAULT_REPO` set, cwd is not a repo, and `default_repo` is also configured | Env var wins — it's a session-scoped override, checked before the more permanent config setting |
| `PMT_DEFAULT_REPO` unset or empty | No effect; falls through to `default_repo` (or the "not inside a git repository" error) exactly as before Phase 8a |

## Deliberately out of scope

- Push, PR/issue API integration (GitHub/GitLab) — purely local git for now, and still deferred as of the Phase 7 v2 work (not selected for implementation).
- ~~`pmt close` / issue cleanup~~ — implemented as Phase 7b (`pmt close`/`pmt reopen` with an append-only archive workflow, see doc/templates.md and doc/commands.md).
- ~~Config-editing subcommands (`pmt repo add/list/remove`)~~ — implemented as Phase 7a: `pmt repo add/list/remove/set-default`, see doc/commands.md.
- ~~Bare-repo support~~ — implemented as Phase 7c, see the row above and doc/architecture.md.
- ~~Template sharing between repos~~ — implemented as Phase 7d, see the row above, doc/templates.md, and doc/commands.md.
- Release pipeline — `go install` only.
