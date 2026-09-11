package tileset

import (
	"encoding/json"
	"os"

	mbtiles "github.com/brendan-ward/mbtiles-go"
	"github.com/consbio/mbtileserver/handlers"
)

func ReadMBTilesInfo(filename, baseDir string) (MapInfo, error) {
	id, err := handlers.RelativePathID(filename, baseDir)
	if err != nil {
		return MapInfo{}, err
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return MapInfo{}, err
	}

	db, err := mbtiles.Open(filename)
	if err != nil {
		return MapInfo{}, err
	}
	defer db.Close()

	meta, err := db.ReadMetadata()
	if err != nil {
		return MapInfo{}, err
	}

	info := MapInfo{
		ID:       id,
		Name:     id,
		Kind:     KindMBTiles,
		Format:   db.GetTileFormat().String(),
		Size:     stat.Size(),
		Modified: stat.ModTime(),
		Path:     filename,
		TileJSON: "/services/" + id,
	}
	if v, ok := meta["name"].(string); ok && v != "" {
		info.Name = v
	}
	if v, ok := meta["description"].(string); ok {
		info.Description = v
	}
	if v, ok := meta["attribution"].(string); ok {
		info.Attribution = v
	}
	if v, ok := meta["minzoom"].(int); ok {
		info.MinZoom = v
	}
	if v, ok := meta["maxzoom"].(int); ok {
		info.MaxZoom = v
	}
	if v, ok := meta["bounds"].([]float64); ok {
		info.Bounds = v
	}
	if v, ok := meta["center"].([]float64); ok {
		info.Center = v
	}
	if v, ok := meta["vector_layers"]; ok {
		raw, err := json.Marshal(v)
		if err == nil {
			info.VectorLayers = raw
		}
	}
	return info, nil
}
