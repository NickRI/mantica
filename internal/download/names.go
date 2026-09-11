package download

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

func ArchiveExt(raw string) string {
	u, err := url.Parse(raw)
	p := raw
	if err == nil {
		p = u.Path
	}
	lower := strings.ToLower(p)
	for _, ext := range []string{".tar.gz", ".tgz", ".pmtiles.gz", ".mbtiles.gz", ".gz", ".bz2", ".zip"} {
		if strings.HasSuffix(lower, ext) {
			switch ext {
			case ".pmtiles.gz", ".mbtiles.gz":
				return ".gz"
			default:
				return ext
			}
		}
	}
	return ""
}

func StripArchiveExt(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return name[:len(name)-7]
	case strings.HasSuffix(lower, ".tgz"):
		return name[:len(name)-4]
	case strings.HasSuffix(lower, ".gz"):
		return name[:len(name)-3]
	case strings.HasSuffix(lower, ".bz2"):
		return name[:len(name)-4]
	case strings.HasSuffix(lower, ".zip"):
		return name[:len(name)-4]
	default:
		return name
	}
}

func TilesetExtFromName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".pmtiles"):
		return ".pmtiles"
	case strings.HasSuffix(lower, ".mbtiles"):
		return ".mbtiles"
	default:
		return ""
	}
}

func FinalTilesetName(remote string) string {
	name := SanitizeName(StripArchiveExt(remote))
	if TilesetExtFromName(name) != "" {
		return name
	}
	return name + ".mbtiles"
}

func RemoteFilename(raw, override string) string {
	ext := ArchiveExt(raw)
	if override != "" {
		name := SanitizeName(override)
		if ext == "" {
			return FinalTilesetName(name)
		}
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ext) || (ext == ".gz" && strings.HasSuffix(lower, ".gz")) {
			return name
		}
		return FinalTilesetName(name) + ext
	}
	u, err := url.Parse(raw)
	if err != nil {
		if ext == "" {
			return "tiles.mbtiles"
		}
		return "tiles.mbtiles" + ext
	}
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		if ext == "" {
			name = "tiles.mbtiles"
		} else {
			name = "tiles.mbtiles" + ext
		}
	}
	return SanitizeName(name)
}

func SanitizeName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	if name == "" || name == "." {
		return "tiles.mbtiles"
	}
	return name
}
