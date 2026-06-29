// sfsymbols validates and searches SF Symbols by reading the system's own
// catalog at /System/Library/CoreServices/CoreGlyphs.bundle — the same data
// the SF Symbols app and UIKit use. No brute-force UIImage(systemName:)
// probing, and we get per-OS availability a runtime check can't provide.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"howett.net/plist"
)

const bundle = "/System/Library/CoreServices/CoreGlyphs.bundle/Contents/Resources"

type availability struct {
	Symbols       map[string]string            `plist:"symbols"`
	YearToRelease map[string]map[string]string `plist:"year_to_release"`
}

// catalog holds names + years as sorted parallel slices: check binary-searches
// names, and search reads the year by index for free during its scan. Sorted
// slices gob-decode ~2.4x faster than a map (no hashing).
type catalog struct {
	names    []string                     // sorted symbol names
	years    []string                     // release year, parallel to names
	releases map[string]map[string]string // year -> platform -> version
	keywords map[string][]string          // name -> search terms (search only)
	aliases  map[string]string            // alias -> canonical name
}

// coreCache is the gob-serialized form of the availability data.
type coreCache struct {
	Names    []string
	Years    []string
	Releases map[string]map[string]string
}

func loadPlist(name string, out interface{}) error {
	f, err := os.Open(filepath.Join(bundle, name))
	if err != nil {
		return err
	}
	defer f.Close()
	return plist.NewDecoder(f).Decode(out)
}

// yearOf binary-searches the sorted name table.
func (c *catalog) yearOf(name string) (string, bool) {
	i := sort.SearchStrings(c.names, name)
	if i < len(c.names) && c.names[i] == name {
		return c.years[i], true
	}
	return "", false
}

// loadCatalog loads only the plists a command needs. check skips the 142KB
// keyword plist (withKeywords=false); only search parses it. Each plist is
// served from a derived gob cache when fresh (see cache.go).
func loadCatalog(withKeywords bool) (*catalog, error) {
	var core coreCache
	if err := cachedLoad("name_availability.plist", "core", &core, func() error {
		var av availability
		if err := loadPlist("name_availability.plist", &av); err != nil {
			return err
		}
		core.Releases = av.YearToRelease
		core.Names = make([]string, 0, len(av.Symbols))
		for k := range av.Symbols {
			core.Names = append(core.Names, k)
		}
		sort.Strings(core.Names)
		core.Years = make([]string, len(core.Names))
		for i, n := range core.Names {
			core.Years[i] = av.Symbols[n]
		}
		return nil
	}); err != nil {
		return nil, err
	}
	c := &catalog{names: core.Names, years: core.Years, releases: core.Releases}
	if err := cachedLoad("name_aliases.strings", "alias", &c.aliases,
		func() error { return loadPlist("name_aliases.strings", &c.aliases) }); err != nil {
		return nil, err
	}
	if withKeywords {
		if err := cachedLoad("symbol_search.plist", "kw", &c.keywords,
			func() error { return loadPlist("symbol_search.plist", &c.keywords) }); err != nil {
			return nil, err
		}
	}
	return c, nil
}

var platformOrder = []string{"iOS", "macOS", "watchOS", "tvOS", "visionOS"}

// since returns "iOS 14.0+" for a symbol, or "" if missing.
func (c *catalog) since(name, platform string) string {
	year, ok := c.yearOf(name)
	if !ok {
		return ""
	}
	if ver, ok := c.releases[year][platform]; ok {
		return fmt.Sprintf("%s %s+", platform, ver)
	}
	return "release " + year
}

// versions returns per-platform availability for a symbol (every OS), in a
// stable order. Empty if the symbol is missing.
func (c *catalog) versions(name string) []string {
	year, ok := c.yearOf(name)
	if !ok {
		return nil
	}
	rel := c.releases[year]
	out := make([]string, 0, len(platformOrder))
	for _, p := range platformOrder {
		if ver, ok := rel[p]; ok {
			out = append(out, fmt.Sprintf("%s %s+", p, ver))
		}
	}
	return out
}

// score ranks a symbol against a query. Higher is better; 0 means no match.
// Name matches are reliable; keyword matches are whole-word only — raw
// substring matching on keywords is noise (e.g. "van" inside "advanced").
//
// Allocation-free: this runs once per symbol per query (~9k/query), so it
// scans for delimited segments instead of strings.Split / strings.Fields.
func score(q, name string, terms []string) int {
	switch {
	case name == q:
		return 100
	case hasSegment(name, '.', q, false):
		return 90
	case strings.HasPrefix(name, q) && len(name) > len(q) && name[len(q)] == '.':
		return 80
	}
	for _, t := range terms {
		if t == q {
			return 70
		}
	}
	for _, t := range terms {
		if hasSegment(t, ' ', q, false) {
			return 60
		}
	}
	if hasSegment(name, '.', q, true) {
		return 45
	}
	if strings.Contains(name, q) {
		return 30
	}
	return 0
}

// hasSegment reports whether q is a sep-delimited segment of s (prefix=false),
// or whether some segment starts with q (prefix=true). No allocation.
func hasSegment(s string, sep byte, q string, prefix bool) bool {
	for i := 0; i <= len(s); {
		seg := s[i:]
		if j := strings.IndexByte(seg, sep); j >= 0 {
			seg = seg[:j]
			i += j + 1
		} else {
			i = len(s) + 1
		}
		if prefix {
			if strings.HasPrefix(seg, q) {
				return true
			}
		} else if seg == q {
			return true
		}
	}
	return false
}

func isVariant(name string) bool {
	return strings.HasSuffix(name, ".ar") || strings.HasSuffix(name, ".hi")
}

// ---- commands ----

type checkResult struct {
	Name         string   `json:"name"`
	Exists       bool     `json:"exists"`
	Canonical    string   `json:"canonical,omitempty"`
	Since        string   `json:"since,omitempty"`        // single platform
	Availability []string `json:"availability,omitempty"` // --platform all
}

func cmdCheck(c *catalog, opt options) int {
	allPlatforms := opt.platform == "all"
	var out []checkResult
	allExist := true
	for _, name := range opt.args {
		canonical := c.aliases[name]
		target := name
		if canonical != "" {
			target = canonical
		}
		_, exists := c.yearOf(target)
		if !exists {
			allExist = false
		}
		r := checkResult{Name: name, Exists: exists}
		if canonical != "" && canonical != name {
			r.Canonical = canonical
		}
		if exists {
			if allPlatforms {
				r.Availability = c.versions(target)
			} else {
				r.Since = c.since(target, opt.platform)
			}
		}
		out = append(out, r)
	}
	if opt.jsonOut {
		printJSON(out)
	} else {
		for _, r := range out {
			mark := "✓"
			if !r.Exists {
				mark = "✗"
			}
			line := mark + " " + r.Name
			if r.Canonical != "" {
				line += "  → alias of " + r.Canonical
			}
			switch {
			case len(r.Availability) > 0:
				line += "  " + strings.Join(r.Availability, " · ")
			case r.Since != "":
				line += "  (" + r.Since + ")"
			case !r.Exists:
				line += "  not found"
			}
			fmt.Println(line)
		}
	}
	if allExist {
		return 0
	}
	return 1
}

// readStdinNames pulls whitespace/newline-separated symbol names from stdin.
func readStdinNames() ([]string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(data)), nil
}

type searchResult struct {
	Name     string   `json:"name"`
	Since    string   `json:"since,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	year     string
	score    int
}

// sinceYear formats a known release year for a platform (no lookup needed).
func (c *catalog) sinceYear(year, platform string) string {
	if ver, ok := c.releases[year][platform]; ok {
		return fmt.Sprintf("%s %s+", platform, ver)
	}
	return "release " + year
}

func cmdSearch(c *catalog, opt options) int {
	q := strings.ToLower(opt.query)
	// Scan ~9k symbols in parallel; reads of the shared maps/slices are
	// concurrent-safe (no writes). The sort below makes order deterministic.
	w := runtime.GOMAXPROCS(0)
	chunk := (len(c.names) + w - 1) / w
	parts := make([][]searchResult, w)
	var wg sync.WaitGroup
	for k := 0; k < w; k++ {
		lo := k * chunk
		hi := lo + chunk
		if lo > len(c.names) {
			lo = len(c.names)
		}
		if hi > len(c.names) {
			hi = len(c.names)
		}
		wg.Add(1)
		go func(k, lo, hi int) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				name := c.names[i]
				if opt.noVariants && isVariant(name) {
					continue
				}
				terms := c.keywords[name]
				if s := score(q, name, terms); s > 0 {
					parts[k] = append(parts[k], searchResult{Name: name, year: c.years[i], score: s, Keywords: terms})
				}
			}
		}(k, lo, hi)
	}
	wg.Wait()
	var results []searchResult
	for _, p := range parts {
		results = append(results, p...)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		if len(results[i].Name) != len(results[j].Name) {
			return len(results[i].Name) < len(results[j].Name)
		}
		return results[i].Name < results[j].Name
	})
	if len(results) > opt.limit {
		results = results[:opt.limit]
	}
	for i := range results {
		results[i].Since = c.sinceYear(results[i].year, opt.platform)
	}

	if opt.jsonOut {
		for i := range results {
			if !opt.keywords {
				results[i].Keywords = nil
			}
		}
		printJSON(results)
		return 0
	}
	if len(results) == 0 {
		fmt.Printf("no matches for %q\n", opt.query)
		return 1
	}
	suffix := func(r searchResult) string {
		av := r.Since
		if av == "" {
			av = "?"
		}
		s := fmt.Sprintf("%s  (%s)", r.Name, av)
		if opt.keywords && len(r.Keywords) > 0 {
			s += "  [" + strings.Join(r.Keywords, ", ") + "]"
		}
		return s
	}
	if opt.render {
		if proto := graphicsProto(); proto != "" {
			names := make([]string, len(results))
			for i, r := range results {
				names[i] = r.Name
			}
			if pngs, err := renderPNGs(names, opt.renderSize*renderDPR, opt.renderColor, opt.renderWeight); err == nil {
				w := os.Stdout
				cw, ch, _ := cellSize()
				for i, r := range results {
					if i < len(pngs) && pngs[i] != nil {
						cols := imageCols(pngs[i], opt.renderSize, cw, ch)
						emitImage(w, proto, pngs[i], opt.renderSize)
						fmt.Fprintf(w, "\x1b[%dC ", cols+1)
					}
					fmt.Fprint(w, suffix(r))
					fmt.Fprint(w, strings.Repeat("\n", opt.renderSize))
				}
				return 0
			} else {
				fmt.Fprintln(os.Stderr, "render unavailable:", err)
			}
		} else {
			fmt.Fprintln(os.Stderr, "note: terminal has no inline-image support")
		}
	}
	for _, r := range results {
		fmt.Println(suffix(r))
	}
	return 0
}

func printJSON(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// ---- arg parsing ----

type options struct {
	platform     string
	jsonOut      bool
	noVariants   bool
	keywords     bool
	limit        int
	query        string
	args         []string
	render       bool   // search: show glyphs inline
	renderColor  string // glyph tint, rrggbb
	renderSize   int    // glyph height in terminal rows
	renderWeight string // symbol weight (regular..bold)
}

var validPlatforms = map[string]bool{
	"iOS": true, "macOS": true, "watchOS": true, "tvOS": true, "visionOS": true,
	"all": true, // check only: show every platform
}

const usage = `sfsymbols — validate and search SF Symbols from the system catalog

Usage:
  sfsymbols check [flags] <name>...      validate exact symbol name(s)
  sfsymbols search [flags] <query>       find symbols by name or keyword
  sfsymbols render [flags] <name>...     draw the glyphs inline (Ghostty/kitty/iTerm2)

  check and render read names from stdin when none are given as args:
    cat names.txt | sfsymbols check --platform all

Flags:
  --platform <p>   iOS|macOS|watchOS|tvOS|visionOS (default iOS);
                   "all" (check only) shows every platform's version
  --json           machine-readable output
  --limit <n>      max search results (default 20)
  --keywords       show matched keywords (search)
  --no-variants    hide .ar/.hi localized variants (search)
  --render         draw each result's glyph inline (search)
  --color <rrggbb> glyph tint for rendering (default ffffff)
  --size <rows>    glyph height in terminal rows (default 1, matches text)
  --weight <w>     symbol weight: regular|medium|semibold|bold|... (default semibold)

Exit codes: check returns 1 if any name is missing.`

func parse(argv []string) (string, options, error) {
	opt := options{platform: "iOS", limit: 20, renderColor: "ffffff", renderSize: 1, renderWeight: "semibold"}
	if len(argv) == 0 {
		return "", opt, fmt.Errorf("no command")
	}
	cmd := argv[0]
	if cmd != "check" && cmd != "search" && cmd != "render" {
		return "", opt, fmt.Errorf("unknown command %q", cmd)
	}
	var positional []string
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		switch a {
		case "--json":
			opt.jsonOut = true
		case "--no-variants":
			opt.noVariants = true
		case "--keywords":
			opt.keywords = true
		case "--render":
			opt.render = true
		case "--color":
			i++
			if i >= len(argv) {
				return "", opt, fmt.Errorf("--color needs an rrggbb hex value")
			}
			opt.renderColor = strings.TrimPrefix(argv[i], "#")
		case "--size":
			i++
			if i >= len(argv) {
				return "", opt, fmt.Errorf("--size needs a number of rows")
			}
			if _, err := fmt.Sscanf(argv[i], "%d", &opt.renderSize); err != nil || opt.renderSize < 1 {
				return "", opt, fmt.Errorf("--size needs a positive number of rows")
			}
		case "--weight":
			i++
			if i >= len(argv) {
				return "", opt, fmt.Errorf("--weight needs a value (regular, medium, semibold, bold, ...)")
			}
			opt.renderWeight = argv[i]
		case "--platform":
			i++
			if i >= len(argv) || !validPlatforms[argv[i]] {
				return "", opt, fmt.Errorf("--platform needs one of iOS|macOS|watchOS|tvOS|visionOS|all")
			}
			opt.platform = argv[i]
		case "--limit":
			i++
			if i >= len(argv) {
				return "", opt, fmt.Errorf("--limit needs a number")
			}
			if _, err := fmt.Sscanf(argv[i], "%d", &opt.limit); err != nil {
				return "", opt, fmt.Errorf("--limit needs a number")
			}
		default:
			if strings.HasPrefix(a, "--") {
				return "", opt, fmt.Errorf("unknown flag %q", a)
			}
			positional = append(positional, a)
		}
	}
	if cmd == "check" || cmd == "render" {
		// No names on argv → read them from stdin (supports piping a list).
		if len(positional) == 0 {
			names, err := readStdinNames()
			if err != nil {
				return "", opt, fmt.Errorf("reading stdin: %w", err)
			}
			positional = names
		}
		if len(positional) == 0 {
			return "", opt, fmt.Errorf("%s needs at least one name (as args or via stdin)", cmd)
		}
		opt.args = positional
	} else {
		if opt.platform == "all" {
			return "", opt, fmt.Errorf("--platform all is only valid for check")
		}
		if len(positional) != 1 {
			return "", opt, fmt.Errorf("search needs exactly one query")
		}
		opt.query = positional[0]
	}
	return cmd, opt, nil
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		fmt.Println(usage)
		return
	}
	cmd, opt, err := parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "\n"+usage)
		os.Exit(2)
	}
	c, err := loadCatalog(cmd == "search") // only search needs the keyword plist
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading SF Symbols catalog:", err)
		os.Exit(3)
	}
	switch cmd {
	case "check":
		os.Exit(cmdCheck(c, opt))
	case "render":
		os.Exit(cmdRender(c, opt))
	default:
		os.Exit(cmdSearch(c, opt))
	}
}
