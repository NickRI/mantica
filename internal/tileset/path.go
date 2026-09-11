package tileset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func IsPMTilesPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".pmtiles")
}

func Path(dir, id string, kind Kind) string {
	switch kind {
	case KindPMTiles:
		return filepath.Join(dir, id+".pmtiles")
	default:
		return filepath.Join(dir, id+".mbtiles")
	}
}

func DetectKind(dir, id string) (Kind, error) {
	pm := filepath.Join(dir, id+".pmtiles")
	mb := filepath.Join(dir, id+".mbtiles")
	_, errPM := os.Stat(pm)
	_, errMB := os.Stat(mb)
	if errPM == nil && errMB == nil {
		return 0, fmt.Errorf("both mbtiles and pmtiles exist for %s; pass kind=", id)
	}
	if errPM == nil {
		return KindPMTiles, nil
	}
	if errMB == nil {
		return KindMBTiles, nil
	}
	return 0, os.ErrNotExist
}

func KindFromFilename(name string) Kind {
	if strings.HasSuffix(strings.ToLower(name), ".pmtiles") {
		return KindPMTiles
	}
	return KindMBTiles
}

func Validate(path string) error {
	base := filepath.Dir(path)
	if IsPMTilesPath(path) {
		_, err := ReadPMTilesInfo(path, base)
		return err
	}
	_, err := ReadMBTilesInfo(path, base)
	return err
}

// FilterRuntimePaths drops tiles under hidden dirs inside tilesets/.
func FilterRuntimePaths(base string, paths []string) []string {
	out := paths[:0:0]
	for _, p := range paths {
		if isRuntimePath(base, p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isRuntimePath(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "." || part == ".." {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
