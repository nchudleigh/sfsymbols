# sfsymbols

Validate, search, and render SF Symbols from the command line — by reading
macOS's own catalog, not by brute-forcing `UIImage(systemName:)`.

```
$ sfsymbols check van.fill bus.fill hatchback.fill car.ferry.fill
✗ van.fill        not found
✓ bus.fill        (iOS 14.0+)
✗ hatchback.fill  not found
✓ car.ferry.fill  (iOS 15.0+)
```

## Why

LLMs and humans guess SF Symbol names (`van.fill`, `hatchback.fill`) that don't
exist, then loop over candidates calling `UIImage(systemName:)` to find out.
That's slow, only tells you about the current machine, and can't suggest
alternatives.

`sfsymbols` reads the source of truth directly:
`/System/Library/CoreServices/CoreGlyphs.bundle` — the same data the SF Symbols
app and UIKit use. One lookup gives a yes/no **and** the OS version each symbol
shipped in, across every Apple platform. Catalog: ~9,200 symbols, ~3,200 with
semantic keywords, plus name aliases.

## Install

macOS only — it reads the system SF Symbols catalog.

```sh
# One-liner (downloads the latest universal binary)
curl -fsSL https://raw.githubusercontent.com/nchudleigh/sfsymbols/main/install.sh | sh

# Or with Go
go install github.com/nchudleigh/sfsymbols@latest

# Or from source
git clone https://github.com/nchudleigh/sfsymbols
cd sfsymbols && make install     # -> /usr/local/bin (sudo), or `make build` for a local binary
```

Inline rendering (the `render` command and `search --render`) needs the Xcode
Command Line Tools (`xcode-select --install`) for the one-time Swift helper
compile. `check` and `search` work without them.

## Usage

```sh
# Validate names — exits non-zero if any are missing
sfsymbols check car.fill trash.slash made.up.symbol

# Version support across every Apple platform
sfsymbols check bus.fill car.ferry.fill --platform all
# ✓ bus.fill       iOS 14.0+ · macOS 11.0+ · watchOS 7.0+ · tvOS 14.0+ · visionOS 1.0+
# ✓ car.ferry.fill iOS 15.0+ · macOS 12.0+ · watchOS 8.0+ · tvOS 15.0+ · visionOS 1.0+

# Discover what DOES exist for a concept (name + semantic keyword search)
sfsymbols search car --no-variants --limit 5
sfsymbols search wifi --keywords

# Render the real glyphs inline (Ghostty, kitty, WezTerm, iTerm2)
sfsymbols render star.fill heart.fill car.fill
sfsymbols search car --render --no-variants

# Pipe a list — one process for the whole batch
echo "star.fill heart.fill bogus.symbol" | sfsymbols check

# Machine-readable
sfsymbols search trash --json --limit 5

# Aliases resolve to their canonical name
sfsymbols check 123.rectangle
# ✓ 123.rectangle  → alias of numbers.rectangle  (iOS 18.0+)
```

`check` and `render` take names as args or, when given none, read
whitespace/newline-separated names from stdin — a whole list is one process.

### Flags

| flag | applies to | meaning |
|------|-----------|---------|
| `--platform <p>` | check, search | `iOS` (default), `macOS`, `watchOS`, `tvOS`, `visionOS`; `all` (check only) shows every platform |
| `--json` | check, search | machine-readable output |
| `--limit <n>` | search | max results (default 20) |
| `--keywords` | search | show the matched semantic keywords |
| `--no-variants` | search | hide `.ar` / `.hi` localized variants |
| `--render` | search | draw each result's glyph inline |
| `--size <rows>` | render, search | glyph height in terminal rows (default 1, matches text) |
| `--weight <w>` | render, search | `regular`…`bold`…`black` (default `semibold`) |
| `--color <rrggbb>` | render, search | glyph tint (default `ffffff`) |

`check` exits non-zero if any name is missing — drop it in a pre-commit hook or
an agent's tool loop to validate symbol names without a build.

## How it works

Three files in `CoreGlyphs.bundle` are the source of truth:

- `name_availability.plist` — every name + release year → per-OS version
- `symbol_search.plist` — semantic keywords per symbol
- `name_aliases.strings` — alias → canonical name

Search ranks name matches above keyword matches, and keyword matches are
whole-word only (raw substring matching is noise — `van` would otherwise match
`advanced`). If a concept has no symbol (e.g. `minivan`), search says so rather
than guessing; search the parent concept (`car`, `bus`) for real options.

Rendering rasterizes the real glyph via AppKit (a small embedded Swift helper,
compiled once and cached) and emits it with the terminal's inline-image
protocol — Kitty graphics (Ghostty/kitty/WezTerm) or iTerm2.

## Performance

| call | time |
|------|------|
| `check` one name | ~3.2 ms |
| `check` 500 names (stdin) | ~3.5 ms |
| `search` | ~4.4 ms |

Both are within ~1–2 ms of bare Go process startup. The catalog is parsed once
and cached as gob under `~/Library/Caches/sfsymbols`, keyed by the source
file's mtime+size, so it rebuilds automatically when macOS updates the bundle.
Availability is stored as sorted parallel slices (binary search for `check`,
index lookup for `search`), and the ~9k-symbol search scan is parallel and
allocation-free.

## License

MIT © Neil Chudleigh
