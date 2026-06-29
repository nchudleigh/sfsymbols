---
name: sfsymbols
description: Verify and find SF Symbol names with the `sfsymbols` CLI instead of guessing. Use whenever writing or editing code that names an SF Symbol — Image(systemName:), UIImage(systemName:)/NSImage(systemSymbolName:), Label(_:systemImage:), .tabItem, UIApplicationShortcutIcon, etc. — to confirm the name exists and is available on the deployment target, or to find the right symbol for a concept. Stops hallucinated symbol names from shipping and avoids burning tokens on guess-and-check loops.
---

# sfsymbols

`sfsymbols` reads the macOS SF Symbols catalog (`CoreGlyphs.bundle`) directly, so it gives a definitive yes/no on a name plus the OS version that added it. Use it instead of guessing names or writing a `UIImage(systemName:)` probing loop.

## Why this saves tokens

The usual ways an agent handles SF Symbols are expensive:

- **Guess, build, read the error, fix, repeat.** Each wrong name costs a full build cycle and a wall of compiler/runtime output in your context.
- **Write a probe loop** that prints whether each candidate exists, run it, and read the dump.
- **Recall the catalog from memory** — thousands of names you don't actually know, so you guess anyway.

One `sfsymbols check` call replaces all of that with a few lines of verified output. `search` returns about ten ranked matches instead of you trying to enumerate the catalog. Smaller context, fewer round trips, and you never ship a name that doesn't exist.

## When to use it

- **Before** you put any SF Symbol name into code, run `sfsymbols check <name>...` on every name you're about to use.
- When you need a symbol for an idea but don't know the exact name, run `sfsymbols search <concept>`.
- When the project has a minimum OS target, confirm the symbol existed that far back with `--platform`.

## Setup

Needs the `sfsymbols` binary on PATH (macOS only). If `command -v sfsymbols` fails, install it once:

```sh
go install github.com/nchudleigh/sfsymbols@latest
# or: curl -fsSL https://raw.githubusercontent.com/nchudleigh/sfsymbols/main/install.sh | sh
```

## Commands

Validate names (exit code is 0 only if every name exists):

```sh
sfsymbols check car.fill heart.circle made.up.symbol
# ✓ car.fill         (iOS 13.0+)
# ✓ heart.circle     (iOS 13.0+)
# ✗ made.up.symbol   not found
```

Find a symbol for a concept:

```sh
sfsymbols search trash --no-variants --limit 8
sfsymbols search wifi --keywords          # show why each matched
```

Check availability against a deployment target:

```sh
sfsymbols check fly.fill --platform iOS   # ✗ if it didn't exist on the target OS
sfsymbols check bus.fill --platform all   # every platform's first version
```

Machine-readable output for any command: add `--json`.

Batch a long list through stdin (one process, one result block):

```sh
echo "star.fill heart.fill bogus.symbol" | sfsymbols check --json
```

## Workflow for agents

1. Collect every SF Symbol name in the code you're writing or editing.
2. Run `sfsymbols check` on all of them in one call (pass them as args, or pipe via stdin).
3. For any that come back `not found`, run `sfsymbols search <concept>` and pick a real name from the results. Don't substitute another guess.
4. If the project targets an older OS, re-check the chosen names with `--platform <target>` and pick one available that far back.
