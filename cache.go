package main

// Derived cache of the parsed catalog. The bundle plists only change on OS
// updates, so we gob-encode each parsed plist keyed by its source file's
// mtime+size. A gob decode is ~4x faster than reflection-decoding the binary
// plist. If the system bundle changes, the stamp changes and we rebuild —
// the cache is always a faithful mirror of the source of truth.

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "sfsymbols")
}

// stamp identifies a source plist by mtime + size; "" if it can't be stat'd.
func stamp(plistFile string) string {
	fi, err := os.Stat(filepath.Join(bundle, plistFile))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", fi.ModTime().UnixNano(), fi.Size())
}

func loadGob(path string, v interface{}) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	return gob.NewDecoder(f).Decode(v) == nil
}

// saveGob writes v atomically (temp + rename) and prunes stale caches of the
// same kind. Best-effort: a read-only cache dir is not an error.
func saveGob(dir, kind, current string, v interface{}) {
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, kind+"-*.tmp")
	if err != nil {
		return
	}
	if gob.NewEncoder(tmp).Encode(v) != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return
	}
	tmp.Close()
	os.Rename(tmp.Name(), current)
	if olds, err := filepath.Glob(filepath.Join(dir, kind+"-*.gob")); err == nil {
		for _, p := range olds {
			if p != current {
				os.Remove(p)
			}
		}
	}
}

// cachedLoad returns v from cache for plistFile, else runs parse() and caches
// the result. Falls back to a plain parse if the source can't be stamped.
func cachedLoad(plistFile, kind string, v interface{}, parse func() error) error {
	s := stamp(plistFile)
	if s == "" {
		return parse()
	}
	dir := cacheDir()
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.gob", kind, s))
	if loadGob(path, v) {
		return nil
	}
	if err := parse(); err != nil {
		return err
	}
	saveGob(dir, kind, path, v)
	return nil
}
