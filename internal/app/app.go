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
	"time"

	mbtiles "github.com/brendan-ward/mbtiles-go"
	"github.com/consbio/mbtileserver/handlers"
	"github.com/protomaps/go-pmtiles/pmtiles"
	"golang.org/x/time/rate"
)

type App struct {
	dir          string // data root (-dir)
	tilesDir     string // <dir>/tilesets
	version      string
	commit       string
	svc          *handlers.ServiceSet
	pmtiles      *pmtiles.Server
	mu           sync.Mutex
	jobs         map[string]*Download
	cancels      map[string]context.CancelFunc
	downloadWG   sync.WaitGroup
	hashes       hashStore
	verifyCancel map[string]context.CancelFunc
	verifyWG     sync.WaitGroup
	shuttingDown bool
	limiter      *rate.Limiter
	settings     Settings
	catalog      []CatalogSection
	geocoderKeys map[string]string
	geo          GeoSearch
	geoStatus    map[string]geoStatus
	geoCache     *geoCacheStore
}

type Settings struct {
	Language     string         `json:"language"`
	Servers      []RemoteServer `json:"servers"`
	RateLimitBps int64          `json:"rate_limit_bps"`
	Geocoders    []GeocoderCard `json:"geocoders"`
}

type RemoteServer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type MapInfo struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
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
	HashStatus   string          `json:"hash_status,omitempty"`
	HashActual   string          `json:"hash_actual,omitempty"`
	HashError    string          `json:"hash_error,omitempty"`
	HashWritten  int64           `json:"hash_written,omitempty"`
	HashTotal    int64           `json:"hash_total,omitempty"`
	HashChecked  time.Time       `json:"hash_checked,omitempty"`
}

func newApp(dir, geocoderKeysPath string, geocodeCacheBytes int64, version, commit string) (*App, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// TODO: remove migrateLayout in next release.
	migrateLayout(dir)
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

	pmt, err := newPMTilesServer(tilesDir)
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
		jobs:         make(map[string]*Download),
		cancels:      make(map[string]context.CancelFunc),
		verifyCancel: make(map[string]context.CancelFunc),
		settings: Settings{
			Language:  "ru",
			Servers:   []RemoteServer{},
			Geocoders: defaultGeocoderCards(),
		},
		catalog:      loadCatalog(),
		geocoderKeys: loadGeocoderKeys(geocoderKeysPath),
		geoStatus:    map[string]geoStatus{},
		geoCache:     openGeoCache(filepath.Join(dir, "geocode-cache.gz"), geocodeCacheBytes),
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
	a.settings.Geocoders = normalizeGeocoderCards(a.settings.Geocoders)
}

func (a *App) rebuildGeo() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.Geocoders = normalizeGeocoderCards(a.settings.Geocoders)
	a.geo = wrapGeoCache(GeoSearches(buildProviders(a.settings.Geocoders, a.geocoderKeys)), a.geoCache)
}

func (a *App) setGeoStatus(id, status, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.geoStatus[id] = geoStatus{Status: status, Error: errMsg}
}

func (a *App) geocoderCardsPublic() []GeocoderCard {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]GeocoderCard, len(a.settings.Geocoders))
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
	filenames = filterRuntimePaths(a.tilesDir, filenames)

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
	if isPMTilesPath(filename) {
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

func (a *App) removeMap(id, kind string) error {
	if kind == "" {
		var err error
		kind, err = detectMapKind(a.tilesDir, id)
		if err != nil {
			return err
		}
	}
	path := tilesetPath(a.tilesDir, id, kind)
	a.forgetHash(filepath.Base(path))
	if kind == "pmtiles" {
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

func (a *App) listMaps() ([]MapInfo, error) {
	mbFiles, err := mbtiles.FindMBtiles(a.tilesDir)
	if err != nil {
		return nil, err
	}
	mbFiles = filterRuntimePaths(a.tilesDir, mbFiles)
	pmFiles, err := findPMTiles(a.tilesDir)
	if err != nil {
		return nil, err
	}

	out := make([]MapInfo, 0, len(mbFiles)+len(pmFiles))
	for _, filename := range mbFiles {
		info, err := readMapInfo(filename, a.tilesDir)
		if err != nil {
			out = append(out, brokenMapInfo(filename, a.tilesDir, "mbtiles", err))
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
		info, err := readPMTilesInfo(filename, a.tilesDir)
		if err != nil {
			out = append(out, brokenMapInfo(filename, a.tilesDir, "pmtiles", err))
			continue
		}
		out = append(out, info)
	}
	for i := range out {
		a.decorateHash(&out[i])
	}
	return out, nil
}

func brokenMapInfo(filename, baseDir, kind string, err error) MapInfo {
	id, idErr := handlers.RelativePathID(filename, baseDir)
	if idErr != nil {
		id = filepath.Base(filename)
	}
	info := MapInfo{
		ID:    id,
		Name:  displayNameFromID(id),
		Kind:  kind,
		Path:  filename,
		Error: err.Error(),
	}
	if kind == "mbtiles" {
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

func readMapInfo(filename, baseDir string) (MapInfo, error) {
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
		Kind:     "mbtiles",
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

func shortID(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:6])
}

// filterRuntimePaths drops tiles under hidden dirs inside tilesets/.
func filterRuntimePaths(base string, paths []string) []string {
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
