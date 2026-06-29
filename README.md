# sfsymbols

Check, search, and render SF Symbols from the terminal. It reads the catalog macOS already ships, so you get a real answer instead of guessing names.

```
$ sfsymbols check van.fill bus.fill hatchback.fill car.ferry.fill
✗ van.fill        not found
✓ bus.fill        (iOS 14.0+)
✗ hatchback.fill  not found
✓ car.ferry.fill  (iOS 15.0+)
```

## Why

If you've ever asked an LLM for an SF Symbol, you've seen it confidently invent `van.fill` or `hatchback.fill`, then write a loop that calls `UIImage(systemName:)` on a dozen guesses to see which ones stick. That's slow, it only knows about the Mac it's running on, and when a name is wrong it has nothing better to offer.

The names already live on your disk, in `/System/Library/CoreServices/CoreGlyphs.bundle` — the same files the SF Symbols app and UIKit read. `sfsymbols` reads them too. So a check is just a lookup: does the name exist, and which OS version added it, for each Apple platform. About 9,200 symbols, 3,200 of them with search keywords, plus the name aliases.

For a coding agent that's also a token win. One `check` call returns a few lines of verified output instead of the agent guessing a name, building, reading the compiler error, and trying again — or printing a probe loop's output into its context.

## Install

macOS only, since it reads the system catalog.

```sh
curl -fsSL https://raw.githubusercontent.com/nchudleigh/sfsymbols/main/install.sh | sh
```

That installs the binary and, if you use Claude Code, the [agent skill](#use-it-with-coding-agents) too (skip it with `SFSYMBOLS_SKILL=0`).

<details>
<summary>Other ways to install</summary>

```sh
go install github.com/nchudleigh/sfsymbols@latest   # with Go

git clone https://github.com/nchudleigh/sfsymbols   # from source
cd sfsymbols && make install                        # -> /usr/local/bin (sudo)
```

</details>

The `render` command (and `search --render`) needs the Xcode Command Line Tools (`xcode-select --install`) the first time, to compile a tiny Swift helper. `check` and `search` don't need them.

## Usage

```sh
# Validate names. Exits non-zero if any are missing, so it drops into scripts.
sfsymbols check car.fill trash.slash made.up.symbol

# Which OS version added it, on every platform
sfsymbols check bus.fill car.ferry.fill --platform all
# ✓ bus.fill       iOS 14.0+ · macOS 11.0+ · watchOS 7.0+ · tvOS 14.0+ · visionOS 1.0+
# ✓ car.ferry.fill iOS 15.0+ · macOS 12.0+ · watchOS 8.0+ · tvOS 15.0+ · visionOS 1.0+

# Find what actually exists for an idea (matches names and keywords)
sfsymbols search car --no-variants --limit 5
sfsymbols search wifi --keywords

# Draw the real glyphs inline (Ghostty, kitty, WezTerm, iTerm2)
sfsymbols render star.fill heart.fill car.fill
sfsymbols search car --render --no-variants

# Pipe a list and pay startup once
echo "star.fill heart.fill bogus.symbol" | sfsymbols check

# JSON for tooling
sfsymbols search trash --json --limit 5

# Aliases resolve to the real name
sfsymbols check 123.rectangle
# ✓ 123.rectangle  → alias of numbers.rectangle  (iOS 18.0+)
```

`check` and `render` take names as arguments, or read them from stdin when you don't pass any, so a whole list runs in one process.

### Flags

| flag | applies to | meaning |
|------|-----------|---------|
| `--platform <p>` | check, search | `iOS` (default), `macOS`, `watchOS`, `tvOS`, `visionOS`; `all` (check only) shows every platform |
| `--json` | check, search | machine-readable output |
| `--limit <n>` | search | max results (default 20) |
| `--keywords` | search | show the keywords that matched |
| `--no-variants` | search | hide `.ar` / `.hi` localized variants |
| `--render` | search | draw each result's glyph inline |
| `--size <rows>` | render, search | glyph height in terminal rows (default 1, matches text) |
| `--weight <w>` | render, search | `regular` through `black` (default `semibold`) |
| `--color <rrggbb>` | render, search | glyph tint (default `ffffff`) |

## Use it with coding agents

Point Claude Code (or Cursor, Copilot, etc.) at `sfsymbols` so it checks a name before writing it into Swift. It stops invented names from shipping, and it's cheaper than guess-build-fix: one call, a few lines back, done.

### Claude Code skill

The repo ships a skill that Claude Code runs on its own when it reaches for a symbol. The install one-liner above already sets it up. To add it on its own:

```sh
mkdir -p ~/.claude/skills/sfsymbols
curl -fsSL https://raw.githubusercontent.com/nchudleigh/sfsymbols/main/skill/sfsymbols/SKILL.md \
  -o ~/.claude/skills/sfsymbols/SKILL.md
```

Start a new Claude Code session and it picks the skill up, or invoke it directly with `/sfsymbols`. For one project only, put it in that project's `.claude/skills/sfsymbols/` instead. (From a clone of this repo, `make install-skill` does the same thing.)

### Other agents

Drop a rule into your project's `AGENTS.md` / `.cursorrules` / `CLAUDE.md` telling the agent to verify names with `sfsymbols check` before writing them. The exact snippet is in [`skill/README.md`](skill/README.md).

## How it works

Three files in `CoreGlyphs.bundle` do the work:

- `name_availability.plist` — every name and the release that added it, mapped to OS versions
- `symbol_search.plist` — keywords per symbol
- `name_aliases.strings` — alias to canonical name

Search puts name matches ahead of keyword matches, and keyword matches have to be whole words, otherwise `van` matches `advanced` and you get junk. When nothing exists for what you typed (say `minivan`), it tells you instead of stretching to a bad guess. Search the broader idea (`car`, `bus`) to see the real options.

`render` asks AppKit to rasterize the actual glyph through a small Swift helper, which gets compiled once and cached. The PNG goes out over whatever inline-image protocol your terminal speaks: Kitty graphics for Ghostty, kitty, and WezTerm, or the iTerm2 protocol.

## Performance

| call | time |
|------|------|
| `check` one name | ~3.2 ms |
| `check` 500 names (stdin) | ~3.5 ms |
| `search` | ~4.4 ms |

That's close enough to bare process startup that there isn't much left to shave. The catalog parses once and caches to `~/Library/Caches/sfsymbols` as gob, keyed by the source file's mtime and size, so it rebuilds itself whenever macOS updates the bundle. Availability lives in sorted parallel slices (binary search for `check`, index lookup for `search`), and the search scan runs across every core without allocating.

## License

MIT © Neil Chudleigh
