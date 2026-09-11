package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const tilesetsDirName = "tilesets"

// migrateLayout rewrites the pre-split data layout into:
//
//	<root>/tilesets/              — *.mbtiles / *.pmtiles
//	<root>/settings.json
//	<root>/downloads.json
//	<root>/hashes.json
//	<root>/geocode-cache.gz
//	<root>/downloads/             — hydra workdirs
//
// TODO: remove in next release.
func migrateLayout(root string) {
	tiles := filepath.Join(root, tilesetsDirName)
	if err := os.MkdirAll(tiles, 0o755); err != nil {
		panic(err)
	}

	// State may still live inside the old tiles directory (when -dir became the parent).
	migrateFile(filepath.Join(tiles, ".settings.json"), filepath.Join(root, "settings.json"))
	migrateFile(filepath.Join(tiles, ".downloads.json"), filepath.Join(root, "downloads.json"))
	migrateFile(filepath.Join(tiles, ".hashes.json"), filepath.Join(root, "hashes.json"))
	migrateFile(filepath.Join(tiles, ".geocode-cache.gz"), filepath.Join(root, "geocode-cache.gz"))
	migrateDir(filepath.Join(tiles, ".downloads"), filepath.Join(root, "downloads"))

	// Or flat old root: -dir pointed at the tiles folder itself.
	migrateFile(filepath.Join(root, ".settings.json"), filepath.Join(root, "settings.json"))
	migrateFile(filepath.Join(root, ".downloads.json"), filepath.Join(root, "downloads.json"))
	migrateFile(filepath.Join(root, ".hashes.json"), filepath.Join(root, "hashes.json"))
	migrateFile(filepath.Join(root, ".geocode-cache.gz"), filepath.Join(root, "geocode-cache.gz"))
	migrateDir(filepath.Join(root, ".downloads"), filepath.Join(root, "downloads"))

	entries, err := os.ReadDir(root)
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".pmtiles") && !strings.HasSuffix(lower, ".mbtiles") {
			continue
		}
		src := filepath.Join(root, name)
		dest := filepath.Join(tiles, name)
		migrateFile(src, dest)
	}
}

func migrateFile(src, dest string) {
	if src == dest {
		return
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	if _, err := os.Stat(dest); err == nil {
		slog.Warn("layout migrate skip, destination exists", "src", src, "dest", dest)
		return
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		panic(err)
	}
	if err := os.Rename(src, dest); err != nil {
		panic(err)
	}
	slog.Info("layout migrate", "src", src, "dest", dest)
}

func migrateDir(src, dest string) {
	if src == dest {
		return
	}
	st, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	if !st.IsDir() {
		panic("layout migrate: expected directory: " + src)
	}
	if _, err := os.Stat(dest); err == nil {
		slog.Warn("layout migrate skip, destination exists", "src", src, "dest", dest)
		return
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if err := os.Rename(src, dest); err != nil {
		panic(err)
	}
	slog.Info("layout migrate", "src", src, "dest", dest)
}
