#!/bin/sh
# Install the latest sfsymbols release (macOS universal binary).
#   curl -fsSL https://raw.githubusercontent.com/nchudleigh/sfsymbols/main/install.sh | sh
set -e

REPO="nchudleigh/sfsymbols"
ASSET="sfsymbols"

if [ "$(uname -s)" != "Darwin" ]; then
  echo "sfsymbols is macOS-only (it reads the system SF Symbols catalog)." >&2
  exit 1
fi

# Pick a writable bin directory on PATH.
BINDIR="${BINDIR:-/usr/local/bin}"
if [ ! -d "$BINDIR" ] || [ ! -w "$BINDIR" ]; then
  BINDIR="$HOME/.local/bin"
fi
mkdir -p "$BINDIR"

URL="https://github.com/$REPO/releases/latest/download/$ASSET"
echo "Downloading $ASSET → $BINDIR ..."
curl -fSL -o "$BINDIR/$ASSET" "$URL"
chmod +x "$BINDIR/$ASSET"
xattr -d com.apple.quarantine "$BINDIR/$ASSET" 2>/dev/null || true

echo "Installed $BINDIR/$ASSET"
case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *) echo "Note: add $BINDIR to your PATH." ;;
esac

# Install the Claude Code skill too, if Claude Code is set up. Skip with SFSYMBOLS_SKILL=0.
if [ "${SFSYMBOLS_SKILL:-1}" != "0" ] && [ -d "$HOME/.claude" ]; then
  SKILL="$HOME/.claude/skills/sfsymbols"
  mkdir -p "$SKILL"
  if curl -fsSL -o "$SKILL/SKILL.md" "https://raw.githubusercontent.com/$REPO/main/skill/sfsymbols/SKILL.md"; then
    echo "Installed Claude Code skill → $SKILL"
  fi
fi

echo "Try: sfsymbols search car"
