package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NickRI/mantica/internal/download"
	"github.com/NickRI/mantica/internal/geocode"
	"github.com/NickRI/mantica/internal/hash"
	"github.com/NickRI/mantica/internal/tileset"
	mbtiles "github.com/brendan-ward/mbtiles-go"
	"github.com/consbio/mbtileserver/handlers"
	"github.com/protomaps/go-pmtiles/pmtiles"
	"golang.org/x/time/rate"
)

const tilesetsDirName = "tilesets"

type App struct {
	dir          string // data root (-dir)
	tilesDir     string // <dir>/tilesets
	version      string
	commit       string
	svc          *handlers.ServiceSet
	pmtiles      *pmtiles.Server
	mu           sync.Mutex
	jobs         map[string]*download.Download
	cancels      map[string]context.CancelFunc
	pausing      map[string]bool
	downloadWG   sync.WaitGroup
	hashes       hash.Store
	verifyCancel map[string]context.CancelFunc
	verifyWG     sync.WaitGroup
	shuttingDown bool
	limiter      *rate.Limiter
	settings     Settings
	catalog      []CatalogSection
	geocoderKeys map[string]string
	geo          geocode.GeoSearch
	geoStatus    map[string]geoProviderStatus
	geoCache     *geocode.Cache
}

type geoProviderStatus struct {
	Status geocode.Status
	Error  string
}

type Settings struct {
	Language     string                 `json:"language"`
	Servers      []RemoteServer         `json:"servers"`
	RateLimitBps int64                  `json:"rate_limit_bps"`
	Geocoders    []geocode.GeocoderCard `json:"geocoders"`
}

type RemoteServer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func newApp(dir, geocoderKeysPath string, geocodeCacheBytes int64, version, commit string, catalogJSON []byte) (*App, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tilesDir := filepath.Join(dir, tilesetsDirName)
	if err := os.MkdirAll(tilesDir, 0o755); err != nil {
		return nil, err
	}

	root, err := url.Parse("/services")
	if err != nil {
		return nil, err
	}

	svc, err := handlers.New(&handlers.ServiceSetConfig{
		RootURL:           root,
		EnableServiceList: true,
		EnableTileJSON:    true,
		EnablePreview:     false,
	})
	if err != nil {
		return nil, err
	}

	pmt, err := tileset.NewPMTilesServer(tilesDir)
	if err != nil {
		return nil, err
	}

	a := &App{
		dir:          dir,
		tilesDir:     tilesDir,
		version:      version,
		commit:       commit,
		svc:          svc,
		pmtiles:      pmt,
		jobs:         make(map[string]*download.Download),
		cancels:      make(map[string]context.CancelFunc),
		pausing:      make(map[string]bool),
		verifyCancel: make(map[string]context.CancelFunc),
		settings: Settings{
			Language:  "ru",
			Servers:   []RemoteServer{},
			Geocoders: geocode.DefaultGeocoderCards(),
		},
		catalog:      loadCatalog(catalogJSON),
		geocoderKeys: geocode.LoadGeocoderKeys(geocoderKeysPath),
		geoStatus:    map[string]geoProviderStatus{},
		geoCache:     geocode.OpenCache(filepath.Join(dir, "geocode-cache.gz"), geocodeCacheBytes),
	}
	a.loadSettings()
	a.setRateLimit(a.settings.RateLimitBps)
	a.rebuildGeo()
	a.loadJobs()
	a.loadHashes()
	a.clearOrphanDownloads()
	if err := a.scan(); err != nil {
		return nil, err
	}
	a.resumeJobs()
	return a, nil
}

func (a *App) catalogByID(id string) (CatalogItem, bool) {
	for _, sec := range a.catalog {
		for _, item := range sec.Items {
			if item.ID == id {
				return item, true
			}
		}
	}
	return CatalogItem{}, false
}

func (a *App) settingsPath() string {
	return filepath.Join(a.dir, "settings.json")
}

func (a *App) loadSettings() {
	data, err := os.ReadFile(a.settingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	if err := json.Unmarshal(data, &a.settings); err != nil {
		panic(err)
	}
	if a.settings.Language == "" {
		a.settings.Language = "ru"
	}
	if a.settings.Servers == nil {
		a.settings.Servers = []RemoteServer{}
	}
	a.settings.Geocoders = geocode.NormalizeGeocoderCards(a.settings.Geocoders)
}

func (a *App) rebuildGeo() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.Geocoders = geocode.NormalizeGeocoderCards(a.settings.Geocoders)
	a.geo = geocode.WrapCache(geocode.GeoSearches(geocode.BuildProviders(a.settings.Geocoders, a.geocoderKeys)), a.geoCache)
}

func (a *App) setGeoStatus(id string, status geocode.Status, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.geoStatus[id] = geoProviderStatus{Status: status, Error: errMsg}
}

func (a *App) geocoderCardsPublic() []geocode.GeocoderCard {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]geocode.GeocoderCard, len(a.settings.Geocoders))
	for i, c := range a.settings.Geocoders {
		c.HasKey = a.geocoderKeys[c.ID] != ""
		st := a.geoStatus[c.ID]
		c.Status = st.Status
		c.Error = st.Error
		out[i] = c
	}
	return out
}

func (a *App) saveSettings() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.settingsPath(), data, 0o644)
}

func (a *App) scan() error {
	filenames, err := mbtiles.FindMBtiles(a.tilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	filenames = tileset.FilterRuntimePaths(a.tilesDir, filenames)

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, filename := range filenames {
		id, err := handlers.RelativePathID(filename, a.tilesDir)
		if err != nil {
			slog.Warn("skip mbtiles id", "path", filename, "err", err)
			continue
		}
		if a.svc.HasTileset(id) {
			continue
		}
		if err := a.svc.AddTileset(filename, id); err != nil {
			slog.Warn("skip mbtiles add", "path", filename, "err", err)
			continue
		}
	}
	return nil
}

func (a *App) addFile(filename string) (string, error) {
	id, err := handlers.RelativePathID(filename, a.tilesDir)
	if err != nil {
		return "", err
	}
	if tileset.IsPMTilesPath(filename) {
		return id, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.svc.HasTileset(id) {
		if err := a.svc.UpdateTileset(id); err != nil {
			return "", err
		}
		return id, nil
	}
	if err := a.svc.AddTileset(filename, id); err != nil {
		return "", err
	}
	return id, nil
}

func (a *App) removeMap(id string, kind tileset.Kind) error {
	path := tileset.Path(a.tilesDir, id, kind)
	a.forgetHash(filepath.Base(path))
	if kind == tileset.KindPMTiles {
		return os.Remove(path)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.svc.HasTileset(id) {
		if err := a.svc.RemoveTileset(id); err != nil {
			return err
		}
	}
	return os.Remove(path)
}

func (a *App) resolveKind(id, kindStr string) (tileset.Kind, error) {
	if kindStr == "" {
		return tileset.DetectKind(a.tilesDir, id)
	}
	return tileset.KindString(kindStr)
}

func (a *App) listMaps() ([]tileset.MapInfo, error) {
	mbFiles, err := mbtiles.FindMBtiles(a.tilesDir)
	if err != nil {
		return nil, err
	}
	mbFiles = tileset.FilterRuntimePaths(a.tilesDir, mbFiles)
	pmFiles, err := tileset.FindPMTiles(a.tilesDir)
	if err != nil {
		return nil, err
	}

	out := make([]tileset.MapInfo, 0, len(mbFiles)+len(pmFiles))
	for _, filename := range mbFiles {
		info, err := tileset.ReadMBTilesInfo(filename, a.tilesDir)
		if err != nil {
			out = append(out, tileset.BrokenMapInfo(filename, a.tilesDir, tileset.KindMBTiles, err))
			continue
		}
		if !a.svc.HasTileset(info.ID) {
			if _, err := a.addFile(filename); err != nil {
				info.Error = err.Error()
			}
		}
		out = append(out, info)
	}
	for _, filename := range pmFiles {
		info, err := tileset.ReadPMTilesInfo(filename, a.tilesDir)
		if err != nil {
			out = append(out, tileset.BrokenMapInfo(filename, a.tilesDir, tileset.KindPMTiles, err))
			continue
		}
		out = append(out, info)
	}
	for i := range out {
		a.decorateHash(&out[i])
	}
	return out, nil
}

func shortID(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:6])
}

func (s RemoteServer) origin() string {
	u, err := url.Parse(s.URL)
	if err != nil {
		return strings.TrimRight(s.URL, "/")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.Path = strings.TrimSuffix(u.Path, "/services")
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

func (s RemoteServer) servicesURL() string {
	return s.origin() + "/services"
}
