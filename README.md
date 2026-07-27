# pmtooling

Source for `pmt` — a CLI tool that manages project issues (bugs, features, etc.) as git branches and git worktrees in a separate target repository.

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
go install github.com/<you>/pmtooling/cmd/pmt@latest
```

`pmt` v1 is implemented: `new`, `template new`, `template list`, and `list` all work end-to-end. See `doc/edge-cases.md` for what's explicitly out of scope for v1 (issue cleanup, remote/PR integration, config-editing subcommands).

## Claude Code skill

[.claude/skills/pmt/SKILL.md](.claude/skills/pmt/SKILL.md) teaches Claude how to *use* `pmt` day-to-day (commands, workflows, gotchas) — separate from the `doc/` files below, which document how `pmt` itself is built. Copy the `pmt` folder into `~/.claude/skills/pmt` (available everywhere) or a target repo's `.claude/skills/pmt` (that project only) to use it outside this repo.

The skill folder also carries two deeper reference files: [building-templates.md](.claude/skills/pmt/building-templates.md) (shaping a template's `CLAUDE.md`, permissions, MCP servers, and skills for the issue kind it produces) and [workflow-patterns.md](.claude/skills/pmt/workflow-patterns.md) (turning a fresh issue into a concrete plan, and when to reach for iterative/loop-based work).

## Documentation

- [doc/architecture.md](doc/architecture.md) — repo separation, worktree layout, repo resolution, config
- [doc/commands.md](doc/commands.md) — command reference, exact git operations
- [doc/templates.md](doc/templates.md) — template ref namespace, scaffold files, README metadata schema
- [doc/edge-cases.md](doc/edge-cases.md) — edge-case behavior and implementation checklist
- [doc/external-repos.md](doc/external-repos.md) — recommended pattern for an issue worktree to reach a separate code repo
