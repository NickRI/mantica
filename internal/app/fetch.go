package app

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/consbio/mbtileserver/handlers"
	"github.com/divyam234/hydra"
	"golang.org/x/time/rate"
)

//go:embed catalog.json
var catalogFS embed.FS

type Download struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Name       string    `json:"name"`
	RemoteName string    `json:"remote_name"`
	Checksum   string    `json:"checksum,omitempty"`
	Total      int64     `json:"total"`
	Written    int64     `json:"written"`
	Speed      float64   `json:"speed"` // bytes/sec, instantaneous
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	Created    time.Time `json:"created"`
}

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

const (
	downloadRetries   = 3
	downloadRetryWait = 10 * time.Second
)

func loadCatalog() []CatalogSection {
	data, err := catalogFS.ReadFile("catalog.json")
	if err != nil {
		panic(err)
	}
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
			items[i].Filename = finalTilesetName(remoteFilename(items[i].URL, ""))
		}
		out = append(out, CatalogSection{Category: cat, Items: items})
	}
	return out
}

func archiveExt(raw string) string {
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

func stripArchiveExt(name string) string {
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

func tilesetExtFromName(name string) string {
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

func finalTilesetName(remote string) string {
	name := sanitizeName(stripArchiveExt(remote))
	if tilesetExtFromName(name) != "" {
		return name
	}
	return name + ".mbtiles"
}

func remoteFilename(raw, override string) string {
	ext := archiveExt(raw)
	if override != "" {
		name := sanitizeName(override)
		if ext == "" {
			return finalTilesetName(name)
		}
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ext) || (ext == ".gz" && strings.HasSuffix(lower, ".gz")) {
			return name
		}
		return finalTilesetName(name) + ext
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
	return sanitizeName(name)
}

func sanitizeName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	if name == "" || name == "." {
		return "tiles.mbtiles"
	}
	return name
}

func (a *App) jobsPath() string {
	return filepath.Join(a.dir, "downloads.json")
}

func (a *App) loadJobs() {
	path := a.jobsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("downloads file missing", "path", path)
			return
		}
		panic(err)
	}
	var jobs []*Download
	if err := json.Unmarshal(data, &jobs); err != nil {
		panic(err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, job := range jobs {
		a.jobs[job.ID] = job
	}
	slog.Info("downloads loaded", "path", path, "count", len(jobs))
	for _, job := range jobs {
		slog.Info("download job",
			"id", job.ID,
			"name", job.Name,
			"status", job.Status,
			"written", job.Written,
			"total", job.Total,
			"error", job.Error,
		)
	}
}

func (a *App) saveJobs() {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*Download, 0, len(a.jobs))
	for _, job := range a.jobs {
		out = append(out, job)
	}
	slices.SortFunc(out, func(x, y *Download) int {
		return x.Created.Compare(y.Created)
	})
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(a.jobsPath(), data, 0o644); err != nil {
		panic(err)
	}
}

func (a *App) resumeJobs() {
	a.mu.Lock()
	var resume []*Download
	var skip []*Download
	for _, job := range a.jobs {
		switch job.Status {
		case "running", "paused", "error":
			resume = append(resume, job)
		default:
			skip = append(skip, job)
		}
	}
	a.mu.Unlock()
	slog.Info("download resume scan", "resume", len(resume), "skip", len(skip))
	for _, job := range skip {
		slog.Info("download skip resume", "id", job.ID, "name", job.Name, "status", job.Status)
	}
	for _, job := range resume {
		slog.Info("download resume", "id", job.ID, "name", job.Name, "status", job.Status, "written", job.Written, "total", job.Total, "error", job.Error)
		go a.runDownload(job)
	}
}

func (a *App) pauseAllDownloads() {
	a.mu.Lock()
	a.shuttingDown = true
	cancels := make([]context.CancelFunc, 0, len(a.cancels))
	for _, cancel := range a.cancels {
		cancels = append(cancels, cancel)
	}
	a.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	a.cancelVerifies()
	done := make(chan struct{})
	go func() {
		a.downloadWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Second):
		slog.Warn("downloads did not stop before shutdown timeout")
	}
}

func (a *App) startDownload(rawURL, name, checksum string, replace bool) (*Download, error) {
	remote := remoteFilename(rawURL, name)
	destName := finalTilesetName(remote)
	dest := filepath.Join(a.tilesDir, destName)

	if job := a.claimDownload(destName, replace); job != nil {
		return job, nil
	}

	if _, err := os.Stat(dest); err == nil {
		if !replace {
			return nil, errAlreadyDownloaded
		}
		if err := a.removeTilesetNamed(destName); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if job := a.claimDownload(destName, replace); job != nil {
		return job, nil
	}

	a.mu.Lock()
	var stale []*Download
	for id, job := range a.jobs {
		if job.Name == destName {
			stale = append(stale, job)
			delete(a.jobs, id)
		}
	}
	id := shortID(rawURL + "|" + fmt.Sprint(time.Now().UnixNano()))
	job := &Download{
		ID:         id,
		URL:        rawURL,
		Name:       destName,
		RemoteName: remote,
		Checksum:   checksum,
		Status:     "running",
		Created:    time.Now(),
	}
	a.jobs[id] = job
	a.mu.Unlock()
	for _, old := range stale {
		a.clearHydraWorkDir(old)
	}
	a.saveJobs()
	a.rememberChecksum(destName, a.catalogChecksum(destName, checksum))
	go a.runDownload(job)
	return job, nil
}

func (a *App) claimDownload(destName string, replace bool) *Download {
	a.mu.Lock()
	if existing := a.activeJobByNameLocked(destName); existing != nil {
		a.mu.Unlock()
		return existing
	}
	if !replace {
		if existing := a.resumableJobByNameLocked(destName); existing != nil {
			a.mu.Unlock()
			go a.runDownload(existing)
			return existing
		}
	}
	a.mu.Unlock()
	return nil
}

func (a *App) activeJobByNameLocked(name string) *Download {
	for _, job := range a.jobs {
		if job.Name == name && (job.Status == "running" || job.Status == "paused") {
			return job
		}
	}
	return nil
}

func (a *App) resumableJobByNameLocked(name string) *Download {
	for _, job := range a.jobs {
		if job.Name == name && job.Status == "error" {
			return job
		}
	}
	return nil
}

func (a *App) removeTilesetNamed(name string) error {
	kind := "mbtiles"
	if strings.HasSuffix(strings.ToLower(name), ".pmtiles") {
		kind = "pmtiles"
	}
	id, err := handlers.RelativePathID(filepath.Join(a.tilesDir, name), a.tilesDir)
	if err != nil {
		return err
	}
	return a.removeMap(id, kind)
}

var errAlreadyDownloaded = errors.New("already downloaded")
var errJobNotFound = errors.New("download not found")
var errResumeNotAllowed = errors.New("download cannot be resumed")

func (a *App) resumeDownload(id string) (*Download, error) {
	a.mu.Lock()
	job := a.jobs[id]
	if job == nil {
		a.mu.Unlock()
		return nil, errJobNotFound
	}
	if _, running := a.cancels[job.ID]; running {
		a.mu.Unlock()
		return job, nil
	}
	switch job.Status {
	case "error", "paused":
		a.mu.Unlock()
		go a.runDownload(job)
		return job, nil
	default:
		a.mu.Unlock()
		return nil, errResumeNotAllowed
	}
}

func (a *App) rateLimiter() *rate.Limiter {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.limiter
}

func (a *App) setRateLimit(bps int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.RateLimitBps = bps
	if bps <= 0 {
		a.limiter = nil
		return
	}
	burst := int(bps)
	if burst < 64*1024 {
		burst = 64 * 1024
	}
	a.limiter = rate.NewLimiter(rate.Limit(bps), burst)
}

func (a *App) hydraOptions(onProgress hydra.ProgressFunc) hydra.Options {
	opts := hydra.DefaultOptions()
	opts.Split = 8
	opts.MaxConnectionsPerServer = 8
	opts.Timeout = 48 * time.Hour
	opts.ExistingFile = hydra.ExistingFileResume
	opts.UserAgent = "mantica/0.1"
	opts.OnProgress = onProgress
	base := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   opts.MaxConnectionsPerServer,
		MaxConnsPerHost:       opts.MaxConnectionsPerServer,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    true,
	}
	opts.Transport = &rateTransport{base: base, get: a.rateLimiter}
	return opts
}

func (a *App) hydraWorkDir(job *Download) string {
	return filepath.Join(a.dir, "downloads", job.ID)
}

func (a *App) clearHydraWorkDir(job *Download) {
	dir := a.hydraWorkDir(job)
	slog.Info("download workdir remove", "id", job.ID, "dir", dir)
	_ = os.RemoveAll(dir)
}

func relocateFile(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		slog.Info("tileset renamed", "src", src, "dest", dest)
		return nil
	} else {
		slog.Warn("tileset rename failed, copying", "src", src, "dest", dest, "err", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, in)
	if err != nil {
		out.Close()
		os.Remove(dest)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dest)
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}
	slog.Info("tileset copied", "src", src, "dest", dest, "bytes", n)
	return nil
}

func (a *App) runDownload(job *Download) {
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	if _, running := a.cancels[job.ID]; running {
		a.mu.Unlock()
		cancel()
		return
	}
	a.downloadWG.Add(1)
	a.cancels[job.ID] = cancel
	job.Status = "running"
	job.Error = ""
	a.mu.Unlock()
	a.saveJobs()

	defer func() {
		job.Speed = 0
		a.mu.Lock()
		delete(a.cancels, job.ID)
		a.mu.Unlock()
		cancel()
		a.saveJobs()
		a.downloadWG.Done()
	}()

	workDir := a.hydraWorkDir(job)
	slog.Info("download start", "id", job.ID, "name", job.Name, "url", job.URL, "dir", workDir)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		failDownload(job, "mkdir", err)
		return
	}

	lastSave := time.Now()
	opts := a.hydraOptions(func(p hydra.Progress) {
		job.Total = p.Total
		job.Written = p.Completed
		job.Speed = p.Speed
		job.Error = ""
		if time.Since(lastSave) > 2*time.Second {
			lastSave = time.Now()
			a.saveJobs()
		}
	})

	req := hydra.Request{
		ID:   job.ID,
		URLs: []string{job.URL},
		Dir:  workDir,
		Out:  job.RemoteName,
	}

	var res hydra.Result
	for attempt := 0; attempt <= downloadRetries; attempt++ {
		if attempt > 0 {
			wait := downloadRetryWait << (attempt - 1)
			select {
			case <-ctx.Done():
				a.finishCanceled(job)
				return
			case <-time.After(wait):
			}
			job.Error = ""
		}
		// Stale lock from a killed process blocks resume; we own this job.
		_ = os.Remove(filepath.Join(workDir, job.RemoteName) + ".hydra.lock")
		var err error
		res, err = hydra.Download(ctx, req, opts)
		if err == nil {
			break
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			a.finishCanceled(job)
			return
		}
		job.Error = err.Error()
		if attempt == downloadRetries {
			failDownload(job, "hydra", err)
			return
		}
		wait := downloadRetryWait << attempt
		slog.Warn("download retry", "id", job.ID, "name", job.Name, "attempt", attempt+1, "wait", wait, "err", err)
		a.saveJobs()
	}

	job.Total = res.Size
	job.Written = res.Size
	dest := filepath.Join(a.tilesDir, job.Name)
	downloaded := res.Path
	slog.Info("download finished", "id", job.ID, "name", job.Name, "path", downloaded, "dest", dest, "size", res.Size)
	if archiveExt(job.RemoteName) != "" || archiveExt(job.URL) != "" {
		if err := extractArchive(downloaded, dest); err != nil {
			failDownload(job, "extract", err)
			return
		}
	} else if downloaded != dest {
		if err := relocateFile(downloaded, dest); err != nil {
			failDownload(job, "relocate", err)
			return
		}
	}

	a.clearHydraWorkDir(job)

	if err := validateTileset(dest); err != nil {
		failDownload(job, "validate", err)
		return
	}
	if _, err := a.addFile(dest); err != nil {
		failDownload(job, "add", err)
		return
	}
	job.Status = "done"
	a.rememberChecksum(job.Name, a.catalogChecksum(job.Name, job.Checksum))
	slog.Info("download done", "id", job.ID, "name", job.Name, "path", dest)
}

type rateTransport struct {
	base http.RoundTripper
	get  func() *rate.Limiter
}

func (t *rateTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	lim := t.get()
	if lim == nil {
		return resp, nil
	}
	resp.Body = &rateLimitedBody{ctx: req.Context(), r: resp.Body, lim: lim}
	return resp, nil
}

type rateLimitedBody struct {
	ctx context.Context
	r   io.ReadCloser
	lim *rate.Limiter
}

func (b *rateLimitedBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	remaining := n
	for remaining > 0 {
		chunk := remaining
		if burst := b.lim.Burst(); chunk > burst {
			chunk = burst
		}
		if werr := b.lim.WaitN(b.ctx, chunk); werr != nil {
			return n, werr
		}
		remaining -= chunk
	}
	return n, err
}

func (b *rateLimitedBody) Close() error { return b.r.Close() }

func extractArchive(src, dest string) error {
	slog.Info("extract archive", "src", src, "dest", dest)
	lower := strings.ToLower(src)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(src, dest)
	case strings.HasSuffix(lower, ".gz"):
		return extractGzip(src, dest)
	case strings.HasSuffix(lower, ".bz2"):
		return extractBzip2(src, dest)
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(src, dest)
	default:
		return fmt.Errorf("unsupported archive: %s", src)
	}
}

func extractGzip(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()
	return writeFile(dest, gz)
}

func extractBzip2(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFile(dest, bzip2.NewReader(in))
}

func extractTarGz(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("no .mbtiles/.pmtiles in %s", src)
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if strings.HasSuffix(strings.ToLower(hdr.Name), ".mbtiles") || strings.HasSuffix(strings.ToLower(hdr.Name), ".pmtiles") {
			return writeFile(dest, tr)
		}
	}
}

func extractZip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".mbtiles") || strings.HasSuffix(strings.ToLower(f.Name), ".pmtiles") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			err = writeFile(dest, rc)
			rc.Close()
			return err
		}
	}
	return fmt.Errorf("no .mbtiles/.pmtiles in %s", src)
}

func writeFile(dest string, r io.Reader) error {
	tmp := dest + ".extract"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		slog.Warn("extract rename failed", "src", tmp, "dest", dest, "err", err)
		return err
	}
	slog.Info("extract renamed", "src", tmp, "dest", dest)
	return nil
}

func failDownload(job *Download, stage string, err error) {
	slog.Warn("download failed", "id", job.ID, "name", job.Name, "stage", stage, "err", err)
	job.Status = "error"
	job.Error = err.Error()
}

func (a *App) finishCanceled(job *Download) {
	a.mu.Lock()
	stopping := a.shuttingDown
	a.mu.Unlock()
	if stopping {
		job.Status = "paused"
		slog.Info("download paused", "id", job.ID, "name", job.Name, "written", job.Written, "total", job.Total)
	} else {
		job.Status = "cancelled"
		slog.Info("download cancelled", "id", job.ID, "name", job.Name)
		a.clearHydraWorkDir(job)
	}
	job.Error = ""
}

func (a *App) job(id string) *Download {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.jobs[id]
}

func (a *App) jobList() []*Download {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*Download, 0, len(a.jobs))
	for _, job := range a.jobs {
		out = append(out, job)
	}
	slices.SortFunc(out, func(x, y *Download) int {
		return x.Created.Compare(y.Created)
	})
	return out
}

func (a *App) cancelJob(id string) {
	a.mu.Lock()
	cancel := a.cancels[id]
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) removeJob(id string) {
	a.cancelJob(id)
	a.mu.Lock()
	job := a.jobs[id]
	delete(a.jobs, id)
	delete(a.cancels, id)
	a.mu.Unlock()
	if job != nil {
		a.clearHydraWorkDir(job)
	}
	a.saveJobs()
}

func (a *App) clearCompletedJobs() {
	a.mu.Lock()
	var gone []*Download
	for id, job := range a.jobs {
		switch job.Status {
		case "done", "cancelled", "error":
			delete(a.jobs, id)
			delete(a.cancels, id)
			gone = append(gone, job)
		}
	}
	a.mu.Unlock()
	for _, job := range gone {
		a.clearHydraWorkDir(job)
	}
	a.saveJobs()
}

func validateTileset(path string) error {
	base := filepath.Dir(path)
	if isPMTilesPath(path) {
		_, err := readPMTilesInfo(path, base)
		return err
	}
	_, err := readMapInfo(path, base)
	return err
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
