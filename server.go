package main

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed web/index.html web/app.css web/app.js web/vendor/maplibre-gl web/vendor/protomaps-basemaps web/vendor/protomaps-basemaps-assets
var webFS embed.FS

func run(listen, dir, authUser, authPass, geocoderKeysPath string, geocodeCacheBytes int64) error {
	app, err := newApp(dir, geocoderKeysPath, geocodeCacheBytes)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /api/maps", app.handleListMaps)
	mux.HandleFunc("DELETE /api/maps/{id...}", app.handleDeleteMap)
	mux.HandleFunc("GET /api/catalog", app.handleCatalog)
	mux.HandleFunc("POST /api/catalog/{id}/download", app.handleCatalogDownload)
	mux.HandleFunc("GET /api/downloads", app.handleListDownloads)
	mux.HandleFunc("GET /api/downloads/{id}", app.handleGetDownload)
	mux.HandleFunc("DELETE /api/downloads/{id}", app.handleDeleteDownload)
	mux.HandleFunc("POST /api/downloads/{id}/cancel", app.handleCancelDownload)
	mux.HandleFunc("POST /api/downloads/clear-completed", app.handleClearCompletedDownloads)
	mux.HandleFunc("POST /api/downloads", app.handleStartDownload)
	mux.HandleFunc("GET /api/storage", app.handleStorage)
	mux.HandleFunc("GET /api/settings", app.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", app.handlePutSettings)
	mux.HandleFunc("GET /api/geocoders", app.handleListGeocoders)
	mux.HandleFunc("PUT /api/geocoders", app.handlePutGeocoders)
	mux.HandleFunc("POST /api/geocoders/{id}/test", app.handleTestGeocoder)
	mux.HandleFunc("GET /api/geocode", app.handleGeocode)
	mux.HandleFunc("GET /api/geocode/autocomplete", app.handleGeocodeAutocomplete)
	mux.HandleFunc("GET /api/geocode/reverse", app.handleGeocodeReverse)
	mux.Handle("/services", app.svc.Handler())
	mux.Handle("/services/", app.svc.Handler())
	mux.HandleFunc("/pmtiles/", app.handlePMTiles)

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}
	fileServer := withMJS(http.FileServer(http.FS(static)))
	mux.Handle("GET /app.js", fileServer)
	mux.Handle("GET /app.css", fileServer)
	mux.Handle("GET /vendor/", fileServer)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "index.html")
	})

	var handler http.Handler = mux
	if authUser != "" {
		handler = basicAuth(authUser, authPass, mux)
	}

	srv := &http.Server{Addr: listen, Handler: handler}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", listen, "dir", dir)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-sigCtx.Done():
		slog.Info("shutting down")
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		app.pauseAllDownloads()
		if app.geoCache != nil {
			app.geoCache.close()
		}
		return nil
	}
}

func withMJS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".mjs") {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		next.ServeHTTP(w, r)
	})
}

func basicAuth(user, pass string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="mantica"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

func (a *App) handleListMaps(w http.ResponseWriter, r *http.Request) {
	maps, err := a.listMaps()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, maps)
}

func (a *App) handleDeleteMap(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	kind := r.URL.Query().Get("kind")
	if err := a.removeMap(id, kind); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.catalog)
}

func (a *App) handleCatalogDownload(w http.ResponseWriter, r *http.Request) {
	item, ok := a.catalogByID(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Replace bool `json:"replace"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job, err := a.startDownload(item.URL, "", item.Checksum, req.Replace)
	if err != nil {
		if errors.Is(err, errAlreadyDownloaded) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *App) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.jobList())
}

func (a *App) handleGetDownload(w http.ResponseWriter, r *http.Request) {
	job := a.job(r.PathValue("id"))
	if job == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *App) handleDeleteDownload(w http.ResponseWriter, r *http.Request) {
	a.removeJob(r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	a.cancelJob(r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleStartDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		Name     string `json:"name"`
		Checksum string `json:"checksum"`
		Replace  bool   `json:"replace"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job, err := a.startDownload(req.URL, req.Name, req.Checksum, req.Replace)
	if err != nil {
		if errors.Is(err, errAlreadyDownloaded) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *App) handleClearCompletedDownloads(w http.ResponseWriter, r *http.Request) {
	a.clearCompletedJobs()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleStorage(w http.ResponseWriter, r *http.Request) {
	info, err := a.storageInfo()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	out := a.settings
	a.mu.Unlock()
	out.Geocoders = a.geocoderCardsPublic()
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Language     string         `json:"language"`
		Servers      []RemoteServer `json:"servers"`
		RateLimitBps *int64         `json:"rate_limit_bps"`
		Geocoders    []GeocoderCard `json:"geocoders"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	if req.Language != "" {
		a.settings.Language = req.Language
	}
	if req.Servers != nil {
		a.settings.Servers = req.Servers
	}
	if req.Geocoders != nil {
		a.settings.Geocoders = normalizeGeocoderCards(req.Geocoders)
	}
	var bps *int64
	if req.RateLimitBps != nil {
		bps = req.RateLimitBps
	}
	a.mu.Unlock()
	if bps != nil {
		a.setRateLimit(*bps)
	}
	if req.Geocoders != nil {
		a.rebuildGeo()
	}
	if err := a.saveSettings(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.handleGetSettings(w, r)
}

func (a *App) handleListGeocoders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.geocoderCardsPublic())
}

func (a *App) handlePutGeocoders(w http.ResponseWriter, r *http.Request) {
	var cards []GeocoderCard
	if err := readJSON(r, &cards); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	a.settings.Geocoders = normalizeGeocoderCards(cards)
	a.mu.Unlock()
	a.rebuildGeo()
	if err := a.saveSettings(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.geocoderCardsPublic())
}

func (a *App) handleTestGeocoder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.mu.Lock()
	var card *GeocoderCard
	for i := range a.settings.Geocoders {
		if a.settings.Geocoders[i].ID == id {
			card = &a.settings.Geocoders[i]
			break
		}
	}
	keys := a.geocoderKeys
	a.mu.Unlock()
	if card == nil {
		http.NotFound(w, r)
		return
	}
	providers := buildProviders([]GeocoderCard{{ID: card.ID, Enabled: true, BaseURL: card.BaseURL}}, keys)
	if len(providers) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown geocoder"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	res, err := providers[0].Search(ctx, "Berlin", GeoOpts{Limit: 1, Lang: a.settings.Language})
	if err != nil {
		a.setGeoStatus(id, "fail", err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	a.setGeoStatus(id, "ok", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": res})
}

func (a *App) geoOptsFromRequest(r *http.Request) GeoOpts {
	q := r.URL.Query()
	opts := GeoOpts{Lang: a.settings.Language, Limit: 8}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			opts.Limit = n
		}
	}
	if v := q.Get("lang"); v != "" {
		opts.Lang = v
	}
	if latS, lonS := q.Get("lat"), q.Get("lon"); latS != "" && lonS != "" {
		lat, e1 := strconv.ParseFloat(latS, 64)
		lon, e2 := strconv.ParseFloat(lonS, 64)
		if e1 == nil && e2 == nil {
			opts.Lat, opts.Lon = &lat, &lon
		}
	}
	if vb := q.Get("viewbox"); vb != "" {
		parts := strings.Split(vb, ",")
		if len(parts) == 4 {
			box := make([]float64, 4)
			ok := true
			for i, p := range parts {
				f, err := strconv.ParseFloat(p, 64)
				if err != nil {
					ok = false
					break
				}
				box[i] = f
			}
			if ok {
				opts.ViewBox = box
			}
		}
	}
	return opts
}

func (a *App) handleGeocode(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, errors.New("q required"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res, err := a.geo.Search(ctx, q, a.geoOptsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": res})
}

func (a *App) handleGeocodeAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": []GeoResult{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res, err := a.geo.Autocomplete(ctx, q, a.geoOptsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": res})
}

func (a *App) handleGeocodeReverse(w http.ResponseWriter, r *http.Request) {
	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, errors.New("lat and lon required"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res, err := a.geo.Reverse(ctx, lat, lon, a.geoOptsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": res})
}
