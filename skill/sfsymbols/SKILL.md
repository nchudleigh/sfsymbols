---
name: sfsymbols
description: Verify SF Symbol names with the `sfsymbols` CLI before writing them into code, and find a symbol for a concept. Confirms a name exists and which OS version added it, instead of guessing or running a UIImage(systemName:) probe loop. One call is cheaper than guess-build-fix.
when_to_use: When writing or editing code that names an SF Symbol — Image(systemName:), UIImage(systemName:), NSImage(systemSymbolName:), Label(_:systemImage:), .tabItem, UIApplicationShortcutIcon. Also when you need a symbol for a concept but don't know the exact name.
allowed-tools: Bash(sfsymbols *)
---

# sfsymbols

Verify SF Symbol names against the macOS catalog instead of guessing. Needs the `sfsymbols` binary on PATH — if `command -v sfsymbols` fails, install it with `go install github.com/nchudleigh/sfsymbols@latest`.

## Before writing any SF Symbol name into code

Run `sfsymbols check` on every name. Exit code is 0 only when all exist.

```
sfsymbols check car.fill heart.circle made.up.symbol
```

For any name that comes back `not found`, run `sfsymbols search <concept>` and use a real result. Do not substitute another guess.

## Commands

- `sfsymbols check <name>...` — validate names. Add `--platform <iOS|macOS|watchOS|tvOS|visionOS>` to check availability against the project's deployment target, or `--platform all` for every platform's first version.
- `sfsymbols search <concept>` — find symbols by name or keyword. Flags: `--keywords`, `--no-variants`, `--limit <n>`.
- Add `--json` to any command for structured output. Pipe a whitespace-separated list into `check` to validate many names in one process.
