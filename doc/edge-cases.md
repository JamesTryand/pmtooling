# Edge cases

Explicit v1 behavior for every edge case identified during design. This table doubles as an acceptance checklist — each row should have a corresponding integration test, run against a scratch target repo (never against `pmtooling` itself).

| Edge case | Resolution |
|---|---|
| `pmt new bug` (no title) | Auto-generate a `bug/0001`-style title (see `commands.md`) |
| `<type>` template doesn't exist | Error naming the type, points to `pmt template list` / `pmt template new` |
| `<type>/<title>` branch already exists | Error if the title was user-supplied; auto-retry up to 5x then error if the title was auto-generated |
| Invalid/unsafe characters in `<title>` | `git check-ref-format` (authoritative for git ref rules) plus a Windows reserved-name/trailing-dot-space layer |
| Worktree directory already exists at the target path, but the branch doesn't | Error, no auto-delete; instructs manual cleanup (and `git worktree prune` if it was a stale worktree) |
| Branch exists but its worktree was manually deleted | Detected via `git worktree list --porcelain`'s `prunable` flag, surfaced in `pmt list` |
| Running `pmt` outside any git repo, no `--repo`/config given | Error: pass `--repo <path>` or `--repo <nickname>` |
| Running `pmt` from inside one of its own issue worktrees | `git-dir` vs `git-common-dir` divergence detected; operations always resolve to the derived main repo root regardless of invocation location |
| `pmt template new <name>` when that template already exists | Error naming the template |
| Listing issues with no central manifest | `git for-each-ref` cross-referenced with `git worktree list --porcelain`; metadata read via `git show <branch>:README.md` |
| Hand-edited/corrupted README front matter | Non-fatal parse failure; the row still lists, metadata column shows `<unparseable>` |
| Target repo is bare | Explicit v1 error — the sibling-worktree convention has no defined location for a bare repo |

## Deliberately out of scope for v1

- Push, PR/issue API integration (GitHub/GitLab) — purely local git for now.
- `pmt close` / issue cleanup — the `status` field exists in the README schema for forward compatibility, but no writer for any value other than `open` exists yet.
- Config-editing subcommands (`pmt repo add/list/remove`) — user/repo config is hand-edited YAML.
- Release pipeline — `go install` only.
