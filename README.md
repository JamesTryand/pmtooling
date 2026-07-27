# pmtooling

Source for `pmt` — a CLI tool that manages project issues (bugs, features, etc.) as git branches and git worktrees in a separate target repository.

## What pmt is for

If you've got a bunch of things you want to do with Claude, `pmt` is a straightforward way to do it: everything lives in one git repo, but each piece of work gets its own branch and worktree — so all your working notes and projects sit in a single place, all committed, while staying completely distinct from each other.

It plays nicely with wherever the real work is tracked. With the Azure DevOps MCP set up, workspaces are typically named after the work item — so Claude pulls in the ticket and its comments straight away to figure out what needs doing. Same idea works for Jira, or anything else with an MCP server. Or, using GitHub's own commands and skills instead, you can just as easily do one workspace per issue or pull request.

It's built with Claude Code specifically in mind: a template can carry a `CLAUDE.md`, `.claude/settings.json` permissions, MCP server configs, and project-scoped skills, all tailored to that kind of issue — so `pmt new` doesn't just start a branch, it hands off a fully briefed working environment.

## Mental model

- **Target repo**: any git repository (e.g. a client's project) that `pmt` manages issues in. This repo (`pmtooling`) is only pmt's own source — pmt never operates on it as a target.
- **Issue** = a git branch named `<type>/<title>` (e.g. `bug/dboverflow`), each checked out into its own git worktree next to the target repo.
- **Template** = a git branch (`pmt/template/<type>`) that a new issue is created from — its own README.md, CLAUDE.md, `.gitignore`, and `.claude/settings.json` become the starting point for every issue of that type.

```
pmt new bug/dboverflow
# -> creates branch bug/dboverflow in the target repo, from template pmt/template/bug
# -> checks it out into a sibling worktree, e.g. ../<repo>.worktrees/bug/dboverflow
```

## Install

```
go install github.com/JamesTryand/pmtooling/cmd/pmt@latest
```

`pmt` v1 is implemented: `new`, `template new`, `template list`, and `list` all work end-to-end. See `doc/edge-cases.md` for what's explicitly out of scope for v1 (issue cleanup, remote/PR integration, config-editing subcommands).

## Getting started

A worked walkthrough of the full lifecycle, from a freshly installed `pmt` to closing out a finished issue. Assumes a target repo already exists — here called `widgetco`.

### 1. Point `pmt` at your repo

Either run `pmt` from inside the repo, or register it once with a nickname so you can reference it from anywhere:

```
pmt repo add widgetco /path/to/widgetco
pmt repo set-default widgetco
```

(`--repo widgetco` also works per-command without setting a default. See `doc/commands.md` for the full resolution order, including the `PMT_DEFAULT_REPO` env var.)

### 2. Set up a template

Every issue is scaffolded from a template, so create one per issue *kind* you plan to use. A blank one:

```
pmt template new bug
git worktree add ../widgetco.worktrees/pmt/template/bug pmt/template/bug
cd ../widgetco.worktrees/pmt/template/bug
```

Now shape it for the kind of work a "bug" issue actually involves. For example, `CLAUDE.md`:

```markdown
# Working a bug in widgetco

1. Reproduce the bug first — write a failing test that captures it.
2. Make the minimal change that makes the test pass.
3. Run `npm test` and confirm nothing else broke.
4. Update CHANGELOG.md if the fix is user-facing.
```

Commit it like any other branch:

```
git add -A && git commit -m "Bug template: reproduce -> test -> fix -> verify"
git worktree remove ../widgetco.worktrees/pmt/template/bug
```

See [building-templates.md](.claude/skills/pmt/building-templates.md) for guidance on going further — scoping `.claude/settings.json` permissions, adding MCP servers, or shipping project-scoped skills for that issue kind.

### 3. Start a new issue

```
pmt new bug/dboverflow
# -> bug/dboverflow created from pmt/template/bug
# -> checked out at ../widgetco.worktrees/bug/dboverflow
cd ../widgetco.worktrees/bug/dboverflow
```

The new worktree already has the template's `CLAUDE.md`, permissions, and any other files committed to the template — nothing further to set up.

### 4. Turn a description or external ticket into a clear plan

Real work rarely starts from nothing — usually there's an Azure DevOps work item, a GitHub issue, or a paragraph someone typed in Slack. Rather than diving straight into code from a vague description, hand that raw material to Claude inside the issue's worktree and ask it to run the `better-init` skill:

```
Here's Azure DevOps work item #4821:

<paste the work item description>

Use /better-init to turn this into a clear plan for this issue before we start.
```

(An `gh issue view 123` or `gh pr view 456` output works just as well as the pasted source.) `/better-init` treats the issue's worktree as the target directory, reuses what you already gave it as the project intro instead of asking you to repeat it, asks only for whatever's genuinely missing, and drafts a concise plan — showing it to you for confirmation before writing anything. The result is a scoped, durable starting point for the issue, built the same way this repo's own `CLAUDE.md` was.

For issues substantial enough to need an ongoing task list across multiple sessions (not just a short plan), see [workflow-patterns.md](.claude/skills/pmt/workflow-patterns.md) for initializing one and, if the work is iterative, structuring a loop around it.

### 5. Do the work, then close it out

Commit as you go, same as any other branch. When it's done:

```
pmt close bug/dboverflow
```

This archives the issue (full history preserved in `refs/heads/pmt/archive`) and removes the branch and worktree. If it turns out more work is needed later, `pmt reopen bug/dboverflow` brings it back exactly as it was.

### Coming back to an issue later

`pmt get <type>/<title>` finds an issue's worktree for you — but since a subprocess can't change its parent shell's directory, it only *prints* the path rather than cd-ing itself (hence "get," not "goto"):

```
dir=$(pmt get bug/dboverflow) && cd "$dir"
```

If the issue's closed, it tells you and gives the exact `pmt reopen` command instead of just failing quietly. Run `pmt get` with no argument to resolve whatever branch is currently checked out at your cwd — useful for jumping back to a worktree's root from a subdirectory of it.

## Claude Code skill

[.claude/skills/pmt/SKILL.md](.claude/skills/pmt/SKILL.md) teaches Claude how to *use* `pmt` day-to-day (commands, workflows, gotchas) — separate from the `doc/` files below, which document how `pmt` itself is built. Copy the `pmt` folder into `~/.claude/skills/pmt` (available everywhere) or a target repo's `.claude/skills/pmt` (that project only) to use it outside this repo.

The skill folder also carries two deeper reference files: [building-templates.md](.claude/skills/pmt/building-templates.md) (shaping a template's `CLAUDE.md`, permissions, MCP servers, and skills for the issue kind it produces) and [workflow-patterns.md](.claude/skills/pmt/workflow-patterns.md) (turning a fresh issue into a concrete plan, and when to reach for iterative/loop-based work).

## Documentation

- [doc/architecture.md](doc/architecture.md) — repo separation, worktree layout, repo resolution, config
- [doc/commands.md](doc/commands.md) — command reference, exact git operations
- [doc/templates.md](doc/templates.md) — template ref namespace, scaffold files, README metadata schema
- [doc/edge-cases.md](doc/edge-cases.md) — edge-case behavior and implementation checklist
- [doc/external-repos.md](doc/external-repos.md) — recommended pattern for an issue worktree to reach a separate code repo

## About this mirror

This GitHub repo is a showcase mirror of the primary repo, republished periodically via `scripts/publish-github.sh`. It carries the code, docs, and skill — a few private working-notes files (phase checklists, decision log) referenced in `CLAUDE.md` are intentionally kept off this mirror, so its commit history won't match the primary repo's.
