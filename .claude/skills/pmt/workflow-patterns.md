# Workflow patterns for working an issue

Two recurring questions once a template is more than the bare scaffold: how does a freshly created issue get from "empty README" to a concrete plan of what needs doing, and how should an agent approach issue kinds that need sustained, iterative work rather than a single pass?

## Initializing an issue with a concrete plan

`pmt new` stamps `README.md`'s front matter (`type`, `title`, `branch`, `status`, `created`) but leaves the body exactly as the template shipped it — usually a placeholder like "Describe the issue here." pmt has no way to know what the actual work is, so it deliberately doesn't try to generate a task breakdown itself.

Recommended pattern for a template's `CLAUDE.md`: instruct the agent that on first work in a freshly created issue (i.e. `README.md`'s body is still the unmodified placeholder), it should:

1. Ask the user what specifically needs to be done, if it isn't already obvious from the title/branch name or prior conversation.
2. Rewrite `README.md`'s body with a concrete description — and for anything more than a one-line fix, a checklist of the concrete steps — then commit that change.
3. For genuinely multi-step or multi-session work, consider using the `planning-with-files` skill (if installed) to create `task_plan.md` / `findings.md` / `progress.md` inside the issue's own worktree. That keeps a durable, structured plan that survives `/clear` or a context reset, separate from the short human-readable summary in `README.md`.

This division keeps `pmt list`'s README-derived metadata meaningful (title/status/created stay short and scannable) while letting the issue carry as much planning detail as the work actually needs, right alongside the code, in the same worktree and branch.

## Building loops for iterative work

Some issue kinds benefit from an agent that keeps working across multiple passes rather than stopping after one turn — "keep fixing lint errors until the linter passes," "poll CI until it's green," "iterate until the tests pass." Two complementary mechanisms are available, and a template's `CLAUDE.md` can point the agent at whichever fits:

- **Claude Code's own `/loop`** runs a prompt or slash command on a recurring interval (or lets the model self-pace if no interval is given). Good for polling-shaped work with a clear, repeatable check: "/loop 5m re-run the test suite and fix any new failures."
- **A plan file driving the iteration** (e.g. the `planning-with-files` skill's `task_plan.md`) suits open-ended, multi-step work better than a timer: re-reading the plan each turn keeps the agent oriented on what's actually left, rather than repeating a fixed check on a fixed cadence.

These aren't pmt-specific — they're just what to recommend, in the template's own `CLAUDE.md`, for the kind of work that template's issues typically involve. A one-shot chore template needs neither; a "flaky test" or "long migration" template likely wants one or the other.

## Closing the loop back to pmt

Once the work — looped or not — is done and everything is committed in the issue's worktree, `pmt close <type>/<title>` archives it (merges into `pmt/archive`, then removes the live branch and worktree). If more work turns out to be needed later, `pmt reopen <type>/<title>` restores it with full history intact — any plan file the agent wrote comes back too, since it was never anything more than part of the branch's own tree.
