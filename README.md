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

## Documentation

- [doc/architecture.md](doc/architecture.md) — repo separation, worktree layout, repo resolution, config
- [doc/commands.md](doc/commands.md) — command reference, exact git operations
- [doc/templates.md](doc/templates.md) — template ref namespace, scaffold files, README metadata schema
- [doc/edge-cases.md](doc/edge-cases.md) — edge-case behavior and implementation checklist
