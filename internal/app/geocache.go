package app

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type geoCacheStore struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	order    []string
	entries  map[string]cachedGeoEntry
	dirty    bool
	saveCh   chan struct{}
}

type cachedGeoEntry struct {
	Results []GeoResult `json:"results"`
	Bytes   int         `json:"bytes"`
}

type geoCacheDisk struct {
	Order   []string                  `json:"order"`
	Entries map[string]cachedGeoEntry `json:"entries"`
}

func openGeoCache(path string, maxBytes int64) *geoCacheStore {
	if maxBytes <= 0 {
		return nil
	}
	c := &geoCacheStore{
		path:     path,
		maxBytes: maxBytes,
		entries:  map[string]cachedGeoEntry{},
		saveCh:   make(chan struct{}, 1),
	}
	c.load()
	go c.persistLoop()
	return c
}

func (c *geoCacheStore) load() {
	f, err := os.Open(c.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("geocode cache open", "err", err)
		}
		return
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		slog.Warn("geocode cache gunzip", "err", err)
		return
	}
	defer gz.Close()
	var disk geoCacheDisk
	if err := json.NewDecoder(gz).Decode(&disk); err != nil {
		slog.Warn("geocode cache decode", "err", err)
		return
	}
	if disk.Entries == nil {
		disk.Entries = map[string]cachedGeoEntry{}
	}
	c.order = disk.Order
	c.entries = disk.Entries
	c.evictLocked()
}

func (c *geoCacheStore) persistLoop() {
	for range c.saveCh {
		time.Sleep(2 * time.Second)
		for {
			select {
			case <-c.saveCh:
			default:
				c.flush()
				goto next
			}
		}
	next:
	}
}

func (c *geoCacheStore) scheduleSave() {
	select {
	case c.saveCh <- struct{}{}:
	default:
	}
}

func (c *geoCacheStore) flush() {
	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()
		return
	}
	disk := geoCacheDisk{Order: append([]string{}, c.order...), Entries: map[string]cachedGeoEntry{}}
	for k, v := range c.entries {
		disk.Entries[k] = v
	}
	c.dirty = false
	path := c.path
	c.mu.Unlock()

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		slog.Warn("geocode cache write", "err", err)
		return
	}
	gz := gzip.NewWriter(f)
	enc := json.NewEncoder(gz)
	if err := enc.Encode(disk); err != nil {
		gz.Close()
		f.Close()
		os.Remove(tmp)
		slog.Warn("geocode cache encode", "err", err)
		return
	}
	if err := gz.Close(); err != nil {
		f.Close()
		os.Remove(tmp)
		slog.Warn("geocode cache gzip", "err", err)
		return
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		slog.Warn("geocode cache close", "err", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		slog.Warn("geocode cache rename", "err", err)
	}
}

func (c *geoCacheStore) get(key string) ([]GeoResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.touchLocked(key)
	out := make([]GeoResult, len(ent.Results))
	copy(out, ent.Results)
	return out, true
}

func (c *geoCacheStore) put(key string, results []GeoResult) {
	raw, err := json.Marshal(results)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cachedGeoEntry{Results: results, Bytes: len(raw)}
	c.touchLocked(key)
	c.evictLocked()
	c.dirty = true
	c.scheduleSave()
}

func (c *geoCacheStore) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}

func (c *geoCacheStore) evictLocked() {
	var total int64
	for _, ent := range c.entries {
		total += int64(ent.Bytes)
	}
	for total > c.maxBytes && len(c.order) > 0 {
		old := c.order[0]
		c.order = c.order[1:]
		if ent, ok := c.entries[old]; ok {
			total -= int64(ent.Bytes)
			delete(c.entries, old)
			c.dirty = true
		}
	}
}

func (c *geoCacheStore) close() {
	c.flush()
}

type cachedGeo struct {
	inner GeoSearch
	cache *geoCacheStore
}

func wrapGeoCache(inner GeoSearch, cache *geoCacheStore) GeoSearch {
	if cache == nil {
		return inner
	}
	return &cachedGeo{inner: inner, cache: cache}
}

func (c *cachedGeo) ID() string { return c.inner.ID() }

func geoCacheKey(op, q string, opts GeoOpts, lat, lon float64) string {
	return fmt.Sprintf("%s|%s|%d|%s|%v|%v|%v|%g|%g",
		op, q, opts.Limit, opts.Lang, opts.Lat, opts.Lon, opts.ViewBox, lat, lon)
}

func (c *cachedGeo) Search(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	key := geoCacheKey("search", q, opts, 0, 0)
	if hit, ok := c.cache.get(key); ok {
		slog.Info("geocode cache hit", "op", "search", "q", q)
		return hit, nil
	}
	res, err := c.inner.Search(ctx, q, opts)
	if err == nil {
		c.cache.put(key, res)
	}
	return res, err
}

func (c *cachedGeo) Autocomplete(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	key := geoCacheKey("autocomplete", q, opts, 0, 0)
	if hit, ok := c.cache.get(key); ok {
		slog.Info("geocode cache hit", "op", "autocomplete", "q", q)
		return hit, nil
	}
	res, err := c.inner.Autocomplete(ctx, q, opts)
	if err == nil {
		c.cache.put(key, res)
	}
	return res, err
}

func (c *cachedGeo) Reverse(ctx context.Context, lat, lon float64, opts GeoOpts) ([]GeoResult, error) {
	key := geoCacheKey("reverse", "", opts, lat, lon)
	if hit, ok := c.cache.get(key); ok {
		slog.Info("geocode cache hit", "op", "reverse", "lat", lat, "lon", lon)
		return hit, nil
	}
	res, err := c.inner.Reverse(ctx, lat, lon, opts)
	if err == nil {
		c.cache.put(key, res)
	}
	return res, err
}
