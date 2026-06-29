# sfsymbols as a coding-agent skill

Let an agent verify SF Symbol names instead of guessing them. The agent runs `sfsymbols check` before writing a name into Swift, so it never ships `van.fill`, and it skips the guess-build-fix loop.

It's also a token win. The usual ways an agent handles SF Symbols are expensive: guess a name and read a wall of compiler errors when it's wrong; write a `UIImage(systemName:)` probe loop and read the dump; or try to recall thousands of catalog names it doesn't actually know. One `check` call replaces all of that with a few verified lines, and `search` returns about ten ranked matches instead of an attempt to enumerate the catalog.

Install the binary first (see the main README), then wire up your agent.

## Claude Code

The [main install one-liner](../README.md#install) already drops the skill into `~/.claude/skills/sfsymbols/` when it sees Claude Code. To add it on its own:

```sh
mkdir -p ~/.claude/skills/sfsymbols
curl -fsSL https://raw.githubusercontent.com/nchudleigh/sfsymbols/main/skill/sfsymbols/SKILL.md \
  -o ~/.claude/skills/sfsymbols/SKILL.md
```

For one project only, use that project's `.claude/skills/sfsymbols/SKILL.md` instead and commit it to share with your team. From a clone of this repo, `make install-skill` (or `make install-skill SKILLDIR=.claude/skills`) does the same copy.

This is the standard [Claude Code skills](https://code.claude.com/docs/en/skills) layout: a directory named after the skill with `SKILL.md` inside. Editing an existing skill takes effect live; creating the `skills/` directory for the first time needs a fresh session so it gets watched. Claude loads the skill when it reaches for a symbol, or run `/sfsymbols` directly. The skill pre-approves `Bash(sfsymbols *)`; for a project install that applies once you accept the workspace trust prompt.

## Cursor, Windsurf, Copilot, and others

These read a project rules file (`AGENTS.md`, `.cursorrules`, `CLAUDE.md`, `.github/copilot-instructions.md`). Drop this in:

```md
## SF Symbols

This project uses Apple SF Symbols. Before writing any SF Symbol name into code
(Image(systemName:), UIImage(systemName:), Label(_:systemImage:), etc.), verify
it with the `sfsymbols` CLI — don't guess and don't write a probe loop:

- Validate: `sfsymbols check <name>...` (exits non-zero if any name is missing)
- Discover: `sfsymbols search <concept>`
- Check OS availability: `sfsymbols check <name> --platform <iOS|macOS|...>`

One call returns a verified answer, which is cheaper than guessing, building,
reading the error, and fixing.
```

## Anything that can run a shell command

`sfsymbols ... --json` gives structured output, and `check` returns exit code 0 only when every name exists, so it slots into any tool-use loop or pre-commit hook.
