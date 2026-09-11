package tileset

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/consbio/mbtileserver/handlers"
	"github.com/protomaps/go-pmtiles/pmtiles"
)

func NewPMTilesServer(dir string) (*pmtiles.Server, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	bucket := pmtiles.NewFileBucket(abs)
	server, err := pmtiles.NewServerWithBucket(bucket, "", log.Default(), 64, "/pmtiles")
	if err != nil {
		return nil, err
	}
	server.Start()
	return server, nil
}

func FindPMTiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "." || base == ".." {
				return nil
			}
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".pmtiles") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func ReadPMTilesInfo(filename, baseDir string) (MapInfo, error) {
	id, err := handlers.RelativePathID(filename, baseDir)
	if err != nil {
		return MapInfo{}, err
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return MapInfo{}, err
	}

	f, err := os.Open(filename)
	if err != nil {
		return MapInfo{}, err
	}
	defer f.Close()

	hdr := make([]byte, pmtiles.HeaderV3LenBytes)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return MapInfo{}, fmt.Errorf("read header: %w", err)
	}
	header, err := pmtiles.DeserializeHeader(hdr)
	if err != nil {
		return MapInfo{}, err
	}
	metaEnd := header.MetadataOffset + header.MetadataLength
	if metaEnd > 0 && int64(metaEnd) > stat.Size() {
		return MapInfo{}, fmt.Errorf("truncated pmtiles: need %d bytes, have %d", metaEnd, stat.Size())
	}

	meta := map[string]any{}
	if header.MetadataLength > 0 {
		if _, err := f.Seek(int64(header.MetadataOffset), io.SeekStart); err != nil {
			return MapInfo{}, err
		}
		limited := io.LimitReader(f, int64(header.MetadataLength))
		raw, err := pmtiles.DeserializeMetadataBytes(limited, header.InternalCompression)
		if err != nil {
			return MapInfo{}, fmt.Errorf("metadata: %w", err)
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return MapInfo{}, err
		}
	}

	e7 := 1e7
	info := MapInfo{
		ID:       id,
		Name:     DisplayNameFromID(id),
		Kind:     KindPMTiles,
		Format:   pmtilesFormat(header.TileType),
		MinZoom:  int(header.MinZoom),
		MaxZoom:  int(header.MaxZoom),
		Bounds:   []float64{float64(header.MinLonE7) / e7, float64(header.MinLatE7) / e7, float64(header.MaxLonE7) / e7, float64(header.MaxLatE7) / e7},
		Center:   []float64{float64(header.CenterLonE7) / e7, float64(header.CenterLatE7) / e7, float64(header.CenterZoom)},
		Size:     stat.Size(),
		Modified: stat.ModTime(),
		Path:     filename,
		TileJSON: "/pmtiles/" + id + ".json",
	}
	metaName := ""
	if v, ok := meta["name"].(string); ok {
		metaName = strings.TrimSpace(v)
	}
	if v, ok := meta["description"].(string); ok {
		info.Description = v
	}
	if metaName != "" && !IsGenericMapName(metaName) {
		info.Name = metaName
	} else if metaName != "" && info.Description == "" {
		info.Description = metaName
	}
	if v, ok := meta["attribution"].(string); ok {
		info.Attribution = v
	}
	if v, ok := meta["vector_layers"]; ok {
		raw, err := json.Marshal(v)
		if err == nil {
			info.VectorLayers = raw
		}
	}
	return info, nil
}

func pmtilesFormat(t pmtiles.TileType) string {
	switch t {
	case pmtiles.Mvt:
		return "mvt"
	case pmtiles.Png:
		return "png"
	case pmtiles.Jpeg:
		return "jpg"
	case pmtiles.Webp:
		return "webp"
	case pmtiles.Avif:
		return "avif"
	case pmtiles.Mlt:
		return "mlt"
	default:
		return "pmtiles"
	}
}
