package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	GeocoderNominatim = "nominatim"
	GeocoderPhoton    = "photon"
)

type GeoResult struct {
	Label   string     `json:"label"`
	Lat     float64    `json:"lat"`
	Lon     float64    `json:"lon"`
	BBox    []float64  `json:"bbox,omitempty"` // minLon,minLat,maxLon,maxLat
	Source  string     `json:"source,omitempty"`
}

type GeoOpts struct {
	Limit    int
	Lang     string
	Lat      *float64
	Lon      *float64
	ViewBox  []float64 // minLon,minLat,maxLon,maxLat
}

type GeoSearch interface {
	ID() string
	Search(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error)
	Autocomplete(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error)
	Reverse(ctx context.Context, lat, lon float64, opts GeoOpts) ([]GeoResult, error)
}

type GeocoderCard struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url"`
	HasKey  bool   `json:"has_key"`
	Status  string `json:"status,omitempty"` // ok | fail | ""
	Error   string `json:"error,omitempty"`
}

func defaultGeocoderCards() []GeocoderCard {
	return []GeocoderCard{
		{ID: GeocoderNominatim, Enabled: true, BaseURL: "https://nominatim.openstreetmap.org"},
		{ID: GeocoderPhoton, Enabled: true, BaseURL: "https://photon.komoot.io"},
	}
}

func normalizeGeocoderCards(cards []GeocoderCard) []GeocoderCard {
	defaults := defaultGeocoderCards()
	byID := map[string]GeocoderCard{}
	for _, d := range defaults {
		byID[d.ID] = d
	}
	ordered := make([]GeocoderCard, 0, len(defaults))
	seen := map[string]bool{}
	for _, c := range cards {
		d, ok := byID[c.ID]
		if !ok {
			continue
		}
		if c.BaseURL == "" {
			c.BaseURL = d.BaseURL
		}
		ordered = append(ordered, GeocoderCard{
			ID:      c.ID,
			Enabled: c.Enabled,
			BaseURL: strings.TrimRight(c.BaseURL, "/"),
		})
		seen[c.ID] = true
	}
	for _, d := range defaults {
		if seen[d.ID] {
			continue
		}
		ordered = append(ordered, d)
	}
	return ordered
}

type geoHTTP struct {
	client *http.Client
}

func newGeoHTTP() *geoHTTP {
	return &geoHTTP{client: &http.Client{Timeout: 12 * time.Second}}
}

func redactGeoURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Has("key") {
		q.Set("key", "***")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func (h *geoHTTP) get(ctx context.Context, rawURL string) ([]byte, error) {
	safeURL := redactGeoURL(rawURL)
	slog.Info("geocode request", "url", safeURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mantica/1.0 (local atlas; geocode proxy)")
	req.Header.Set("Accept", "application/json")
	res, err := h.client.Do(req)
	if err != nil {
		slog.Warn("geocode request failed", "url", safeURL, "err", err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		slog.Warn("geocode read failed", "url", safeURL, "status", res.StatusCode, "err", err)
		return nil, err
	}
	slog.Info("geocode response", "url", safeURL, "status", res.StatusCode, "body", string(body))
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

type nominatimProvider struct {
	id, base, key string
	http          *geoHTTP
}

func (p *nominatimProvider) ID() string { return p.id }

func (p *nominatimProvider) Search(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	return p.query(ctx, "/search", q, opts, false)
}

func (p *nominatimProvider) Autocomplete(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	return p.query(ctx, "/search", q, opts, false)
}

func (p *nominatimProvider) Reverse(ctx context.Context, lat, lon float64, opts GeoOpts) ([]GeoResult, error) {
	v := url.Values{}
	v.Set("format", "jsonv2")
	v.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	v.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	v.Set("addressdetails", "1")
	if opts.Lang != "" {
		v.Set("accept-language", opts.Lang)
	}
	if p.key != "" {
		v.Set("key", p.key)
	}
	body, err := p.http.get(ctx, p.base+"/reverse?"+v.Encode())
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if errMsg, ok := raw["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}
	r, ok := parseNominatimItem(raw)
	if !ok {
		return nil, fmt.Errorf("no reverse result")
	}
	r.Source = p.id
	return []GeoResult{r}, nil
}

func (p *nominatimProvider) query(ctx context.Context, path, q string, opts GeoOpts, _ bool) ([]GeoResult, error) {
	v := url.Values{}
	v.Set("q", q)
	v.Set("format", "jsonv2")
	v.Set("addressdetails", "1")
	limit := opts.Limit
	if limit <= 0 {
		limit = 8
	}
	v.Set("limit", strconv.Itoa(limit))
	if opts.Lang != "" {
		v.Set("accept-language", opts.Lang)
	}
	if len(opts.ViewBox) == 4 {
		v.Set("viewbox", fmt.Sprintf("%g,%g,%g,%g", opts.ViewBox[0], opts.ViewBox[3], opts.ViewBox[2], opts.ViewBox[1]))
	}
	if p.key != "" {
		v.Set("key", p.key)
	}
	body, err := p.http.get(ctx, p.base+path+"?"+v.Encode())
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]GeoResult, 0, len(raw))
	for _, item := range raw {
		r, ok := parseNominatimItem(item)
		if !ok {
			continue
		}
		r.Source = p.id
		out = append(out, r)
	}
	return out, nil
}

func parseNominatimItem(item map[string]any) (GeoResult, bool) {
	label, _ := item["display_name"].(string)
	latS, _ := item["lat"].(string)
	lonS, _ := item["lon"].(string)
	lat, err1 := strconv.ParseFloat(latS, 64)
	lon, err2 := strconv.ParseFloat(lonS, 64)
	if label == "" || err1 != nil || err2 != nil {
		return GeoResult{}, false
	}
	r := GeoResult{Label: label, Lat: lat, Lon: lon}
	if bb, ok := item["boundingbox"].([]any); ok && len(bb) == 4 {
		south, e1 := strconv.ParseFloat(fmt.Sprint(bb[0]), 64)
		north, e2 := strconv.ParseFloat(fmt.Sprint(bb[1]), 64)
		west, e3 := strconv.ParseFloat(fmt.Sprint(bb[2]), 64)
		east, e4 := strconv.ParseFloat(fmt.Sprint(bb[3]), 64)
		if e1 == nil && e2 == nil && e3 == nil && e4 == nil {
			r.BBox = []float64{west, south, east, north}
		}
	}
	return r, true
}

type photonProvider struct {
	id, base, key string
	http          *geoHTTP
}

func (p *photonProvider) ID() string { return p.id }

func (p *photonProvider) Search(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	return p.query(ctx, "/api", q, opts)
}

func (p *photonProvider) Autocomplete(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	return p.query(ctx, "/api", q, opts)
}

func (p *photonProvider) Reverse(ctx context.Context, lat, lon float64, opts GeoOpts) ([]GeoResult, error) {
	v := url.Values{}
	v.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	v.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	if lang := photonLang(opts.Lang); lang != "" {
		v.Set("lang", lang)
	}
	if opts.Limit > 0 {
		v.Set("limit", strconv.Itoa(opts.Limit))
	}
	if p.key != "" {
		v.Set("key", p.key)
	}
	body, err := p.http.get(ctx, p.base+"/reverse?"+v.Encode())
	if err != nil {
		return nil, err
	}
	return parsePhotonFeatureCollection(body, p.id)
}

func (p *photonProvider) query(ctx context.Context, path, q string, opts GeoOpts) ([]GeoResult, error) {
	v := url.Values{}
	v.Set("q", q)
	limit := opts.Limit
	if limit <= 0 {
		limit = 8
	}
	v.Set("limit", strconv.Itoa(limit))
	if lang := photonLang(opts.Lang); lang != "" {
		v.Set("lang", lang)
	}
	if opts.Lat != nil && opts.Lon != nil {
		v.Set("lat", strconv.FormatFloat(*opts.Lat, 'f', -1, 64))
		v.Set("lon", strconv.FormatFloat(*opts.Lon, 'f', -1, 64))
	}
	if p.key != "" {
		v.Set("key", p.key)
	}
	body, err := p.http.get(ctx, p.base+path+"?"+v.Encode())
	if err != nil {
		return nil, err
	}
	return parsePhotonFeatureCollection(body, p.id)
}

// photon.komoot.io accepts only default, de, en, fr.
func photonLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "default":
		return "default"
	case "de", "en", "fr":
		return lang
	default:
		return "default"
	}
}

func parsePhotonFeatureCollection(body []byte, source string) ([]GeoResult, error) {
	var fc struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, err
	}
	out := make([]GeoResult, 0, len(fc.Features))
	for _, f := range fc.Features {
		if len(f.Geometry.Coordinates) < 2 {
			continue
		}
		label := photonLabel(f.Properties)
		if label == "" {
			continue
		}
		r := GeoResult{
			Label:  label,
			Lon:    f.Geometry.Coordinates[0],
			Lat:    f.Geometry.Coordinates[1],
			Source: source,
		}
		if ext, ok := f.Properties["extent"].([]any); ok && len(ext) == 4 {
			minLon, e1 := toFloat(ext[0])
			maxLat, e2 := toFloat(ext[1])
			maxLon, e3 := toFloat(ext[2])
			minLat, e4 := toFloat(ext[3])
			if e1 == nil && e2 == nil && e3 == nil && e4 == nil {
				r.BBox = []float64{minLon, minLat, maxLon, maxLat}
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func photonLabel(props map[string]any) string {
	parts := make([]string, 0, 6)
	for _, k := range []string{"name", "housenumber", "street", "district", "city", "county", "state", "country"} {
		if v, ok := props[k].(string); ok && v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}

func toFloat(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case json.Number:
		return t.Float64()
	default:
		return strconv.ParseFloat(fmt.Sprint(v), 64)
	}
}

type geoStatus struct {
	Status string
	Error  string
}

// GeoSearches is a failover chain: first successful provider wins.
type GeoSearches []GeoSearch

func (s GeoSearches) ID() string {
	ids := make([]string, len(s))
	for i, p := range s {
		ids[i] = p.ID()
	}
	return strings.Join(ids, ",")
}

func (s GeoSearches) Search(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("no geocoders enabled")
	}
	var errs []string
	for _, p := range s {
		res, err := p.Search(ctx, q, opts)
		if err != nil {
			errs = append(errs, p.ID()+": "+err.Error())
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
}

func (s GeoSearches) Autocomplete(ctx context.Context, q string, opts GeoOpts) ([]GeoResult, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("no geocoders enabled")
	}
	var errs []string
	for _, p := range s {
		res, err := p.Autocomplete(ctx, q, opts)
		if err != nil {
			errs = append(errs, p.ID()+": "+err.Error())
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
}

func (s GeoSearches) Reverse(ctx context.Context, lat, lon float64, opts GeoOpts) ([]GeoResult, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("no geocoders enabled")
	}
	var errs []string
	for _, p := range s {
		res, err := p.Reverse(ctx, lat, lon, opts)
		if err != nil {
			errs = append(errs, p.ID()+": "+err.Error())
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
}

func buildProviders(cards []GeocoderCard, keys map[string]string) []GeoSearch {
	httpClient := newGeoHTTP()
	out := make([]GeoSearch, 0, len(cards))
	for _, c := range cards {
		if !c.Enabled {
			continue
		}
		key := keys[c.ID]
		switch c.ID {
		case GeocoderNominatim:
			out = append(out, &nominatimProvider{id: c.ID, base: c.BaseURL, key: key, http: httpClient})
		case GeocoderPhoton:
			out = append(out, &photonProvider{id: c.ID, base: c.BaseURL, key: key, http: httpClient})
		}
	}
	return out
}

func loadGeocoderKeys(path string) map[string]string {
	out := map[string]string{}
	if path == "" {
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out
		}
		panic(err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}
