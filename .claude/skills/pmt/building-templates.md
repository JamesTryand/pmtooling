# Building good pmt templates

This goes deeper than `SKILL.md`'s basics: how to shape a template so every issue created from it comes with the right instructions, permissions, tools, and integrations already in place — no pmt command "activates" any of this, it's all just files committed to the template branch. `pmt new` branches from the template's *live* tip, so whatever is on that branch at the moment an issue is created is exactly what the issue inherits.

## Tailor `CLAUDE.md` to the template's kind

Different issue kinds want different guidance, not the generic starter text:

- A **bug** template's `CLAUDE.md` should push toward: reproduce the problem first, write a failing test that captures it, fix, verify the test now passes, check for regressions.
- A **feature** template might push toward: clarify scope with the user before writing code, sketch a plan, implement, test, document.
- A **chore/docs** template can be much lighter — often just "make the change, verify it builds/renders, done."

Whatever the kind, say explicitly in `CLAUDE.md`:
- What "done" looks like for this kind of issue (tests passing? a specific reviewer sign-off? just "builds cleanly"?).
- Any conventions specific to that issue kind that a generic project `CLAUDE.md` wouldn't cover.
- Whether the agent should read anything else first (a linked doc, the code repo's own `CLAUDE.md` if this issue reaches an external repo — see `doc/external-repos.md` in the `pmtooling` source, or the summary in `SKILL.md`).

## Permissions: `.claude/settings.json`

This file travels with the template to every issue of that type, so scope it to what that *kind* of work actually needs — don't just leave it as `{}` (pmt's default) or blanket-allow everything:

- A docs-only template might not need `Bash` at all.
- A bug-fixing template needs whatever test-runner commands that project uses allowed (e.g. `Bash(pytest:*)`, `Bash(npm test:*)`).
- An issue that needs to read/write a separate codebase should grant `permissions.additionalDirectories` pointing at that code repo's path — see `doc/external-repos.md` for the full pattern and why git submodules were deliberately rejected for this. Example:

```json
{
  "permissions": {
    "additionalDirectories": [
      "//C:/work/project-code"
    ]
  }
}
```

This is Claude Code's own setting, not pmt's — pmt's only job is copying the file verbatim from the template into every new issue's worktree.

## MCP servers: `.mcp.json`

A template can ship an `.mcp.json` at its tree root (same level as `README.md`), and it propagates exactly like every other file. Worked example — an Azure DevOps MCP server included in every `bug` template, so the agent can look up the work item matching the issue's branch/title:

```json
{
  "mcpServers": {
    "azure-devops": {
      "command": "npx",
      "args": ["-y", "@azure-devops/mcp-server"],
      "env": {
        "AZURE_DEVOPS_ORG": "your-org",
        "AZURE_DEVOPS_PAT": "${AZURE_DEVOPS_PAT}"
      }
    }
  }
}
```

**Don't commit real credentials into a template branch** — it's git history, shared across every worktree created from it, and hard to fully purge once pushed. Reference an environment variable (as above) or let the MCP server resolve credentials from the OS keychain at runtime instead of writing a literal secret into the file.

## Project-scoped skills: `.claude/skills/`

Same propagation rule applies to `.claude/skills/<name>/SKILL.md` files committed to the template. Worked examples:

- A Python-repo template ships a project-scoped skill teaching the exact test/lint commands and conventions for that codebase, so the agent doesn't have to rediscover them each time.
- A template whose issues routinely involve GitHub PRs ships a `gh`-focused skill with the house conventions for PR descriptions, labels, and review requests.

## Tools: make sure the issue kind's assumptions are documented and allowed

If an issue kind assumes certain CLI tools/runtimes are installed (python, pytest, gh, docker, ...), say so in `CLAUDE.md` — don't make the agent (or a human) rediscover it. And check that back against `.claude/settings.json`'s permission rules: a tightly scoped `Bash` allow-list that doesn't actually include the commands this issue kind needs is a self-inflicted blocker, not a safety win.

## Watch out for: `.claude/` accidentally gitignored

pmt's own scaffolded `.gitignore` doesn't exclude `.claude/`, and the very first scaffold commit is built via raw git plumbing (`hash-object`/`mktree`), which bypasses `.gitignore` entirely — so the initial `.claude/settings.json` is always committed regardless. But once you're editing a template normally (checked out in a worktree, `git add`/`git commit`), ordinary `.gitignore` rules apply again. If you ever replace a template's `.gitignore` with a personal one, don't blanket-exclude `.claude/` — Claude Code's own convention is to gitignore only `.claude/settings.local.json` (personal, machine-specific overrides), not the whole directory. A blanket `.claude/` rule would silently stop any *future* `.claude/` content you add (more skills, updated permissions, an `.mcp.json`) from ever being committed — no error, it just quietly never gets tracked.
