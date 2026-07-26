# Working with external code repos from an issue worktree

## The problem

pmt's target repo manages *issues* — branches, worktrees, README.md metadata (see doc/architecture.md). The actual codebase an issue is about often lives in a **separate** git repository (or several), not the one pmt is tracking issues in. When Claude Code (or a human) is working inside an issue's worktree, it needs to read and write files in that separate code repo.

## Why not submodules or nesting

Git submodules — or otherwise nesting a code repo inside an issue worktree — introduce their own operational complexity: separate commit/checkout state to keep in sync, `.gitmodules` config, detached-HEAD gotchas, and a structural coupling between pmt's issue branches and the code repo's own history that pmt was never designed to manage. Deliberately rejected for this use case.

**Recommended instead**: treat each code repo as a completely independent checkout elsewhere on disk — its own plain branches/worktrees, managed with ordinary git, not by pmt — and grant Claude Code read/write access to it *from* the issue worktree via permissions, not via git structure.

## The mechanism: `permissions.additionalDirectories`

Claude Code's `.claude/settings.json` supports an `additionalDirectories` list under `permissions`, granting file access (Read/Write/Edit/Bash file operations) to directories outside the current project root, in addition to whatever's already accessible under cwd:

```json
{
  "permissions": {
    "additionalDirectories": [
      "//C:/work/project-code"
    ]
  }
}
```

Paths use gitignore-style pattern syntax: `//` prefixes an absolute path, `~/` is home-relative, a bare `/` is project-root-relative. Once added, files under that directory follow the exact same Read/Write/Edit/Bash permission rules (allow/ask/deny) as the working directory itself — no separate permission model to reason about.

This is a genuinely persistent, file-based setting — it survives being committed and copied around, which is exactly what a template branch's `.claude/settings.json` already does for every issue created from it. **No pmt code changes are needed for this pattern to work**: it was verified empirically (Phase 8 scoping) that any file present on a template branch — including `.claude/settings.json` content added by hand after the initial scaffold — flows through automatically to every new issue, since `pmt new` branches from the template's *live* tip rather than copying a fixed file list. Add `additionalDirectories` to a template's `.claude/settings.json` once, and every issue of that type inherits it.

## Caveats

- `additionalDirectories` grants **file access only** — it does not load that other directory's own `CLAUDE.md`, skills, or other config. If Claude also needs the code repo's own guidance, mention that explicitly in the issue template's own `CLAUDE.md` (e.g. "read `<code-repo-path>/CLAUDE.md` if present before making changes").
- The permission rule resolves correctly even though issue worktrees are linked worktrees of pmt's target repo — Claude Code resolves worktree linkage back to the main checkout for permission purposes, so this isn't affected by pmt's own worktree-vs-main-repo distinction (doc/architecture.md#repo-resolution).
- Because a template ships one fixed, committed `settings.json`, this pattern works best when an issue *type* consistently corresponds to one code repo (or a small fixed set) — e.g. a `bug` template scoped to "project X" hardcodes project X's path. If issues of the same type need to reach *different* code repos per issue, this static file-level mechanism can't parameterize that on its own; use per-project template types instead (e.g. `bug-projectx`, `bug-projecty`), or edit `.claude/settings.json` by hand after creating that specific issue.

## The code-repo side: plain branches/worktrees, pmt stays uninvolved

The actual work in a code repo should use its own plain git branches/worktrees — created and managed with ordinary git, not pmt, using whatever branch-naming convention fits that codebase. pmt has no opinion here and doesn't need one: it manages exactly one thing (issues in its own target repo) and stays out of however many other repos an issue happens to reference, matching the repo-separation philosophy already established in doc/architecture.md.
