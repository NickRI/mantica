package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/NickRI/mantica/internal/download"
	"github.com/NickRI/mantica/internal/tileset"
	"github.com/consbio/mbtileserver/handlers"
	"github.com/divyam234/hydra"
	"golang.org/x/time/rate"
)

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
	var jobs []*download.Download
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
	out := make([]*download.Download, 0, len(a.jobs))
	for _, job := range a.jobs {
		out = append(out, job)
	}
	slices.SortFunc(out, func(x, y *download.Download) int {
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
	var resume []*download.Download
	var skip []*download.Download
	for _, job := range a.jobs {
		switch job.Status {
		case download.StatusRunning, download.StatusError:
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

func (a *App) startDownload(rawURL, name, checksum string, replace bool) (*download.Download, error) {
	remote := download.RemoteFilename(rawURL, name)
	destName := download.FinalTilesetName(remote)
	dest := filepath.Join(a.tilesDir, destName)

	if job := a.claimDownload(destName, replace); job != nil {
		return job, nil
	}

	if _, err := os.Stat(dest); err == nil {
		if !replace {
			return nil, download.ErrAlreadyDownloaded
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
	var stale []*download.Download
	for id, job := range a.jobs {
		if job.Name == destName {
			stale = append(stale, job)
			delete(a.jobs, id)
		}
	}
	id := shortID(rawURL + "|" + fmt.Sprint(time.Now().UnixNano()))
	job := &download.Download{
		ID:         id,
		URL:        rawURL,
		Name:       destName,
		RemoteName: remote,
		Checksum:   checksum,
		Status:     download.StatusRunning,
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

func (a *App) claimDownload(destName string, replace bool) *download.Download {
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

func (a *App) activeJobByNameLocked(name string) *download.Download {
	for _, job := range a.jobs {
		if job.Name == name && (job.Status == download.StatusRunning || job.Status == download.StatusPaused) {
			return job
		}
	}
	return nil
}

func (a *App) resumableJobByNameLocked(name string) *download.Download {
	for _, job := range a.jobs {
		if job.Name == name && job.Status == download.StatusError {
			return job
		}
	}
	return nil
}

func (a *App) removeTilesetNamed(name string) error {
	kind := tileset.KindFromFilename(name)
	id, err := handlers.RelativePathID(filepath.Join(a.tilesDir, name), a.tilesDir)
	if err != nil {
		return err
	}
	return a.removeMap(id, kind)
}

func (a *App) resumeDownload(id string) (*download.Download, error) {
	a.mu.Lock()
	job := a.jobs[id]
	if job == nil {
		a.mu.Unlock()
		return nil, download.ErrJobNotFound
	}
	if _, running := a.cancels[job.ID]; running {
		a.mu.Unlock()
		return job, nil
	}
	switch job.Status {
	case download.StatusError, download.StatusPaused:
		a.mu.Unlock()
		go a.runDownload(job)
		return job, nil
	default:
		a.mu.Unlock()
		return nil, download.ErrResumeNotAllowed
	}
}

func (a *App) pauseDownload(id string) (*download.Download, error) {
	a.mu.Lock()
	job := a.jobs[id]
	if job == nil {
		a.mu.Unlock()
		return nil, download.ErrJobNotFound
	}
	cancel := a.cancels[job.ID]
	if cancel == nil || job.Status != download.StatusRunning {
		a.mu.Unlock()
		return nil, download.ErrPauseNotAllowed
	}
	a.pausing[job.ID] = true
	a.mu.Unlock()
	cancel()
	return job, nil
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

func (a *App) hydraWorkDir(job *download.Download) string {
	return filepath.Join(a.dir, "downloads", job.ID)
}

func (a *App) clearHydraWorkDir(job *download.Download) {
	dir := a.hydraWorkDir(job)
	slog.Info("download workdir remove", "id", job.ID, "dir", dir)
	_ = os.RemoveAll(dir)
}

func (a *App) runDownload(job *download.Download) {
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	if _, running := a.cancels[job.ID]; running {
		a.mu.Unlock()
		cancel()
		return
	}
	a.downloadWG.Add(1)
	a.cancels[job.ID] = cancel
	job.Status = download.StatusRunning
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
		download.Fail(job, "mkdir", err)
		return
	}

	lastSave := time.Now()
	opts := download.HydraOptions("mantica/0.1", func(p hydra.Progress) {
		if p.Total > 0 {
			job.Total = p.Total
		}

		if p.Completed > 0 {
			job.Written = p.Completed
		}

		job.Speed = p.Speed
		job.Error = p.Error

		if time.Since(lastSave) > 2*time.Second {
			lastSave = time.Now()
			a.saveJobs()
		}
	}, a.rateLimiter)

	req := hydra.Request{
		ID:   job.ID,
		URLs: []string{job.URL},
		Dir:  workDir,
		Out:  job.RemoteName,
	}

	var res hydra.Result
	for attempt := 0; attempt <= download.Retries; attempt++ {
		if attempt > 0 {
			wait := download.RetryWait << (attempt - 1)
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
		if errors.Is(err, context.Canceled) {
			a.finishCanceled(job)
			return
		}
		job.Error = err.Error()
		if attempt == download.Retries {
			download.Fail(job, "hydra", err)
			return
		}
		wait := download.RetryWait << attempt
		slog.Warn("download retry", "id", job.ID, "name", job.Name, "attempt", attempt+1, "wait", wait, "err", err)
		a.saveJobs()
	}

	job.Total = res.Size
	job.Written = res.Size
	dest := filepath.Join(a.tilesDir, job.Name)
	downloaded := res.Path
	slog.Info("download finished", "id", job.ID, "name", job.Name, "path", downloaded, "dest", dest, "size", res.Size)
	if download.ArchiveExt(job.RemoteName) != "" || download.ArchiveExt(job.URL) != "" {
		if err := download.ExtractArchive(downloaded, dest); err != nil {
			download.Fail(job, "extract", err)
			return
		}
	} else if downloaded != dest {
		if err := download.RelocateFile(downloaded, dest); err != nil {
			download.Fail(job, "relocate", err)
			return
		}
	}

	a.clearHydraWorkDir(job)

	if err := tileset.Validate(dest); err != nil {
		download.Fail(job, "validate", err)
		return
	}
	if _, err := a.addFile(dest); err != nil {
		download.Fail(job, "add", err)
		return
	}
	job.Status = download.StatusDone
	a.rememberChecksum(job.Name, a.catalogChecksum(job.Name, job.Checksum))
	slog.Info("download done", "id", job.ID, "name", job.Name, "path", dest)
}

func (a *App) finishCanceled(job *download.Download) {
	a.mu.Lock()
	pause := a.shuttingDown || a.pausing[job.ID]
	delete(a.pausing, job.ID)
	a.mu.Unlock()
	if pause {
		job.Status = download.StatusPaused
		slog.Info("download paused", "id", job.ID, "name", job.Name, "written", job.Written, "total", job.Total)
	} else {
		job.Status = download.StatusCancelled
		slog.Info("download cancelled", "id", job.ID, "name", job.Name)
		a.clearHydraWorkDir(job)
	}
	job.Error = ""
}

func (a *App) job(id string) *download.Download {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.jobs[id]
}

func (a *App) jobList() []*download.Download {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*download.Download, 0, len(a.jobs))
	for _, job := range a.jobs {
		out = append(out, job)
	}
	slices.SortFunc(out, func(x, y *download.Download) int {
		return x.Created.Compare(y.Created)
	})
	return out
}

func (a *App) cancelJob(id string) {
	a.mu.Lock()
	delete(a.pausing, id)
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
	delete(a.pausing, id)
	a.mu.Unlock()
	if job != nil {
		a.clearHydraWorkDir(job)
	}
	a.saveJobs()
}

func (a *App) clearCompletedJobs() {
	a.mu.Lock()
	var gone []*download.Download
	for id, job := range a.jobs {
		switch job.Status {
		case download.StatusDone, download.StatusCancelled, download.StatusError:
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

func (a *App) clearOrphanDownloads() int {
	root := filepath.Join(a.dir, "downloads")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		panic(err)
	}
	a.mu.Lock()
	keep := map[string]bool{}
	for id, job := range a.jobs {
		switch job.Status {
		case download.StatusRunning, download.StatusPaused, download.StatusError:
			keep[id] = true
		}
	}
	a.mu.Unlock()
	removed := 0
	for _, e := range entries {
		if !e.IsDir() || keep[e.Name()] {
			continue
		}
		dir := filepath.Join(root, e.Name())
		slog.Info("orphan download remove", "dir", dir)
		if err := os.RemoveAll(dir); err != nil {
			panic(err)
		}
		removed++
	}
	if removed > 0 {
		slog.Info("orphan downloads cleared", "removed", removed)
	}
	return removed
}
