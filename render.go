package main

// Inline rendering of real SF Symbol glyphs in the terminal. The system
// rasterizes each glyph (AppKit, via an embedded Swift helper compiled once and
// cached), then we emit the PNGs with the Kitty graphics protocol (Ghostty,
// kitty, WezTerm) or the iTerm2 inline-image protocol.

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

// Terminal cells are ~twice as tall as wide; used to pick a column count that
// preserves each glyph's aspect ratio instead of stretching it.
const cellAspect = 2.0

// Rasterize this many points per display row, so the terminal downscales
// (sharp) rather than upscales (blurry).
const renderDPR = 128

func pngSize(b []byte) (w, h int) {
	if len(b) < 24 {
		return 0, 0
	}
	// IHDR width/height: 8-byte sig + 4 len + 4 "IHDR", then w,h big-endian.
	return int(binary.BigEndian.Uint32(b[16:20])), int(binary.BigEndian.Uint32(b[20:24]))
}

// cellSize queries the terminal for its cell size in pixels (CSI 16 t).
// Returns ok=false if the terminal doesn't answer.
func cellSize() (w, h int, ok bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return 0, 0, false
	}
	defer tty.Close()
	fd := int(tty.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return 0, 0, false
	}
	defer term.Restore(fd, old)

	tty.WriteString("\x1b[16t")
	resp := make(chan string, 1)
	go func() {
		buf := make([]byte, 32)
		n, _ := tty.Read(buf)
		resp <- string(buf[:n])
	}()
	select {
	case s := <-resp:
		// Expect: ESC [ 6 ; <height> ; <width> t
		var hh, ww int
		if _, err := fmt.Sscanf(s, "\x1b[6;%d;%dt", &hh, &ww); err == nil && ww > 0 && hh > 0 {
			return ww, hh, true
		}
	case <-time.After(120 * time.Millisecond):
	}
	return 0, 0, false
}

// imageCols returns the cell width that keeps a PNG's aspect ratio at `rows`,
// using the real cell pixel size when available, else a 2:1 estimate.
func imageCols(png []byte, rows, cellW, cellH int) int {
	w, h := pngSize(png)
	if w == 0 || h == 0 {
		return rows
	}
	ratio := cellAspect
	if cellW > 0 && cellH > 0 {
		ratio = float64(cellH) / float64(cellW)
	}
	cols := int(math.Round(float64(rows) * ratio * float64(w) / float64(h)))
	if cols < 1 {
		cols = 1
	}
	return cols
}

//go:embed render.swift
var renderSwiftSrc string

// graphicsProto reports the terminal's inline-image protocol, or "".
func graphicsProto() string {
	switch {
	case os.Getenv("TERM_PROGRAM") == "iTerm.app":
		return "iterm"
	case strings.Contains(os.Getenv("TERM"), "kitty"),
		strings.Contains(os.Getenv("TERM"), "ghostty"),
		os.Getenv("TERM_PROGRAM") == "ghostty",
		os.Getenv("TERM_PROGRAM") == "WezTerm",
		os.Getenv("KITTY_WINDOW_ID") != "":
		return "kitty"
	}
	return ""
}

// ensureRenderer compiles the embedded Swift helper once, cached by source hash.
func ensureRenderer() (string, error) {
	sum := sha256.Sum256([]byte(renderSwiftSrc))
	dir := cacheDir()
	bin := filepath.Join(dir, fmt.Sprintf("renderer-%x", sum[:6]))
	if fi, err := os.Stat(bin); err == nil && fi.Mode()&0o100 != 0 {
		return bin, nil
	}
	if _, err := exec.LookPath("swiftc"); err != nil {
		return "", fmt.Errorf("swiftc not found (install Xcode Command Line Tools to render)")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	src := filepath.Join(dir, "render.swift")
	if err := os.WriteFile(src, []byte(renderSwiftSrc), 0o644); err != nil {
		return "", err
	}
	out, err := exec.Command("swiftc", "-O", "-o", bin, src).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compiling renderer: %v\n%s", err, out)
	}
	return bin, nil
}

// renderPNGs rasterizes names in one helper run; result[i] is nil if missing.
func renderPNGs(names []string, pt int, color, weight string) ([][]byte, error) {
	bin, err := ensureRenderer()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, fmt.Sprint(pt), color, weight)
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(names))
	var lenBuf [4]byte
	for range names {
		if _, err := io.ReadFull(stdout, lenBuf[:]); err != nil {
			break
		}
		n := binary.BigEndian.Uint32(lenBuf[:])
		if n == 0 {
			out = append(out, nil)
			continue
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(stdout, buf); err != nil {
			break
		}
		out = append(out, buf)
	}
	cmd.Wait()
	return out, nil
}

// emitImage writes a PNG inline at `rows` rows tall. Only the row count is
// given so the terminal preserves the glyph's aspect ratio (no stretch); the
// cursor is left in place (C=1) so the caller positions the label.
func emitImage(w io.Writer, proto string, png []byte, rows int) {
	b64 := base64.StdEncoding.EncodeToString(png)
	switch proto {
	case "iterm":
		fmt.Fprintf(w, "\x1b]1337;File=inline=1;width=auto;height=%d;preserveAspectRatio=1:%s\x07", rows, b64)
	case "kitty":
		const chunkSz = 4096
		keys := fmt.Sprintf("f=100,a=T,C=1,r=%d", rows)
		for i := 0; i < len(b64); i += chunkSz {
			end := i + chunkSz
			if end > len(b64) {
				end = len(b64)
			}
			more := 0
			if end < len(b64) {
				more = 1
			}
			if i == 0 {
				fmt.Fprintf(w, "\x1b_G%s,m=%d;%s\x1b\\", keys, more, b64[i:end])
			} else {
				fmt.Fprintf(w, "\x1b_Gm=%d;%s\x1b\\", more, b64[i:end])
			}
		}
	}
}

func cmdRender(c *catalog, opt options) int {
	proto := graphicsProto()
	names := opt.args

	// Resolve aliases and split out missing symbols up front.
	type item struct {
		name, target string
		alias        bool
	}
	var items []item
	missing := false
	for _, n := range names {
		target := n
		if a := c.aliases[n]; a != "" {
			target = a
		}
		if _, ok := c.yearOf(target); !ok {
			fmt.Printf("✗ %s  not found\n", n)
			missing = true
			continue
		}
		items = append(items, item{name: n, target: target, alias: c.aliases[n] != ""})
	}
	if len(items) == 0 {
		return 1
	}

	if proto == "" {
		fmt.Fprintln(os.Stderr, "note: terminal has no inline-image support; listing names only")
		for _, it := range items {
			fmt.Println("•", it.name)
		}
		return boolExit(!missing)
	}

	targets := make([]string, len(items))
	for i, it := range items {
		targets[i] = it.target
	}
	pngs, err := renderPNGs(targets, opt.renderSize*renderDPR, opt.renderColor, opt.renderWeight)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 3
	}

	w := os.Stdout
	rows := opt.renderSize
	cw, ch, _ := cellSize()
	for i, it := range items {
		if i < len(pngs) && pngs[i] != nil {
			cols := imageCols(pngs[i], rows, cw, ch)
			emitImage(w, proto, pngs[i], rows)
			fmt.Fprintf(w, "\x1b[%dC ", cols+1) // move past image, then a space
		}
		line := it.name
		if it.alias {
			line += "  → " + it.target
		}
		if s := c.since(it.target, "iOS"); s != "" {
			line += "  (" + s + ")"
		}
		fmt.Fprint(w, line)
		fmt.Fprint(w, strings.Repeat("\n", rows)) // advance below the image
	}
	return boolExit(!missing)
}

func boolExit(ok bool) int {
	if ok {
		return 0
	}
	return 1
}
