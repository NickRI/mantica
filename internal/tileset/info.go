package tileset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NickRI/mantica/internal/hash"
	"github.com/consbio/mbtileserver/handlers"
)

type MapInfo struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Kind         Kind            `json:"kind"`
	Description  string          `json:"description,omitempty"`
	Format       string          `json:"format"`
	MinZoom      int             `json:"minzoom"`
	MaxZoom      int             `json:"maxzoom"`
	Bounds       []float64       `json:"bounds,omitempty"`
	Center       []float64       `json:"center,omitempty"`
	Attribution  string          `json:"attribution,omitempty"`
	Size         int64           `json:"size"`
	Modified     time.Time       `json:"modified"`
	Path         string          `json:"path"`
	TileJSON     string          `json:"tilejson"`
	VectorLayers json.RawMessage `json:"vector_layers,omitempty"`
	Error        string          `json:"error,omitempty"`
	Checksum     string          `json:"checksum,omitempty"`
	HashStatus   hash.Status     `json:"hash_status,omitempty"`
	HashActual   string          `json:"hash_actual,omitempty"`
	HashError    string          `json:"hash_error,omitempty"`
	HashWritten  int64           `json:"hash_written,omitempty"`
	HashTotal    int64           `json:"hash_total,omitempty"`
	HashChecked  time.Time       `json:"hash_checked,omitempty"`
}

func BrokenMapInfo(filename, baseDir string, kind Kind, err error) MapInfo {
	id, idErr := handlers.RelativePathID(filename, baseDir)
	if idErr != nil {
		id = filepath.Base(filename)
	}
	info := MapInfo{
		ID:    id,
		Name:  DisplayNameFromID(id),
		Kind:  kind,
		Path:  filename,
		Error: err.Error(),
	}
	if kind == KindMBTiles {
		info.TileJSON = "/services/" + id
	} else {
		info.TileJSON = "/pmtiles/" + id + ".json"
	}
	if st, stErr := os.Stat(filename); stErr == nil {
		info.Size = st.Size()
		info.Modified = st.ModTime()
	}
	return info
}

func DisplayNameFromID(id string) string {
	base := filepath.Base(id)
	replacer := strings.NewReplacer("_", " ", "-", " ", "(", " ", ")", " ", ".", " ", "+", " ")
	parts := strings.Fields(replacer.Replace(base))
	for i, p := range parts {
		if p == "" {
			continue
		}
		lower := strings.ToLower(p)
		switch lower {
		case "odbl", "osm", "mvt", "pmtiles":
			parts[i] = strings.ToUpper(lower)
		default:
			runes := []rune(lower)
			runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
			parts[i] = string(runes)
		}
	}
	if len(parts) == 0 {
		return id
	}
	return strings.Join(parts, " ")
}

func IsGenericMapName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "protomaps basemap", "protomaps", "basemap", "untitled", "map":
		return true
	default:
		return false
	}
}
