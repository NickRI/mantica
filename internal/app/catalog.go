package app

import (
	"encoding/json"

	"github.com/NickRI/mantica/internal/download"
)

type CatalogItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Format      string `json:"format"`
	Region      string `json:"region,omitempty"`
	SizeHint    string `json:"size_hint,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Flag        string `json:"flag,omitempty"`
}

type CatalogSection struct {
	Category string        `json:"category"`
	Items    []CatalogItem `json:"items"`
}

var catalogCategoryOrder = []string{"World", "Europe", "USA", "Asia", "Africa"}

func loadCatalog(data []byte) []CatalogSection {
	var raw map[string][]CatalogItem
	if err := json.Unmarshal(data, &raw); err != nil {
		panic(err)
	}
	out := make([]CatalogSection, 0, len(catalogCategoryOrder))
	for _, cat := range catalogCategoryOrder {
		items := raw[cat]
		if items == nil {
			items = []CatalogItem{}
		}
		for i := range items {
			if items[i].Region == "" {
				items[i].Region = cat
			}
			items[i].Filename = download.FinalTilesetName(download.RemoteFilename(items[i].URL, ""))
		}
		out = append(out, CatalogSection{Category: cat, Items: items})
	}
	return out
}
