# sfsymbols as a coding-agent skill

Let an agent verify SF Symbol names instead of guessing them. The agent runs `sfsymbols check` before writing a name into Swift, so it never ships `van.fill`, and it skips the guess-build-fix loop that burns tokens and context.

First make sure the binary is installed (see the main README), then wire up your agent.

## Claude Code

Copy the skill into your skills directory:

```sh
# from this repo
make install-skill                       # -> ~/.claude/skills/sfsymbols
# or by hand
cp -R skill/sfsymbols ~/.claude/skills/sfsymbols   # user-wide
cp -R skill/sfsymbols .claude/skills/sfsymbols     # this project only
```

Claude Code picks it up on the next session and invokes it on its own when it's about to use an SF Symbol. You can also call it explicitly with `/sfsymbols`.

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
