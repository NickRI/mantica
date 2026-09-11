package app

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/NickRI/mantica/internal/hash"
	"github.com/NickRI/mantica/internal/tileset"
	"github.com/divyam234/hydra"
)

func (a *App) hashesPath() string {
	return filepath.Join(a.dir, "hashes.json")
}

func (a *App) loadHashes() {
	a.hashes = hash.LoadStore(a.hashesPath())
}

func (a *App) saveHashes() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.saveHashesLocked()
}

func (a *App) saveHashesLocked() {
	hash.SaveStore(a.hashesPath(), a.hashes)
}

func (a *App) rememberChecksum(name, checksum string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if checksum != "" {
		a.hashes.Expected[name] = checksum
	} else {
		delete(a.hashes.Expected, name)
	}
	delete(a.hashes.Results, name)
	a.saveHashesLocked()
}

func (a *App) catalogChecksum(name, fallback string) string {
	for _, sec := range a.catalog {
		for _, item := range sec.Items {
			if item.Filename == name {
				return item.Checksum
			}
		}
	}
	return fallback
}

func (a *App) forgetHash(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cancel := a.verifyCancel[name]; cancel != nil {
		cancel()
	}
	delete(a.hashes.Expected, name)
	delete(a.hashes.Results, name)
	a.saveHashesLocked()
}

func (a *App) expectedChecksumLocked(name string) string {
	if v := a.hashes.Expected[name]; v != "" {
		return v
	}
	for _, sec := range a.catalog {
		for _, item := range sec.Items {
			if item.Filename == name && item.Checksum != "" {
				return item.Checksum
			}
		}
	}
	return ""
}

func (a *App) decorateHash(info *tileset.MapInfo) {
	name := filepath.Base(info.Path)
	a.mu.Lock()
	defer a.mu.Unlock()
	info.Checksum = a.expectedChecksumLocked(name)
	rec := a.hashes.Results[name]
	if rec == nil {
		return
	}
	info.HashStatus = rec.Status
	info.HashActual = rec.Actual
	info.HashError = rec.Error
	info.HashWritten = rec.Written
	info.HashTotal = rec.Total
	info.HashChecked = rec.Checked
}

func (a *App) startVerify(id string, kind tileset.Kind) (*hash.Result, error) {
	path := tileset.Path(a.tilesDir, id, kind)
	name := filepath.Base(path)
	a.mu.Lock()
	expected := a.expectedChecksumLocked(name)
	if expected == "" {
		a.mu.Unlock()
		return nil, hash.ErrNoChecksum
	}
	if rec := a.hashes.Results[name]; rec != nil {
		out := *rec
		a.mu.Unlock()
		if rec.Status == hash.StatusRunning {
			return &out, nil
		}
		return &out, hash.ErrAlreadyVerified
	}
	if len(a.verifyCancel) > 0 {
		a.mu.Unlock()
		return nil, hash.ErrVerifyBusy
	}
	if _, err := hydra.ParseChecksum(expected); err != nil {
		a.mu.Unlock()
		return nil, err
	}
	a.hashes.Expected[name] = expected
	prev := a.hashes.Results[name]
	rec := &hash.Result{Expected: expected, Status: hash.StatusRunning, Total: 0}
	if st, err := os.Stat(path); err == nil {
		rec.Total = st.Size()
	}
	a.hashes.Results[name] = rec
	ctx, cancel := context.WithCancel(context.Background())
	a.verifyCancel[name] = cancel
	a.saveHashesLocked()
	a.mu.Unlock()
	a.verifyWG.Add(1)
	go a.runVerify(ctx, cancel, name, path, expected, prev)
	return rec, nil
}

func (a *App) runVerify(ctx context.Context, cancel context.CancelFunc, name, path, expected string, prev *hash.Result) {
	defer a.verifyWG.Done()
	defer func() {
		a.mu.Lock()
		delete(a.verifyCancel, name)
		a.mu.Unlock()
		cancel()
		a.saveHashes()
	}()

	slog.Info("verify start", "name", name, "path", path)
	actual, err := hash.FileProgress(ctx, path, expected, func(written, total int64) {
		a.mu.Lock()
		if rec := a.hashes.Results[name]; rec != nil && rec.Status == hash.StatusRunning {
			rec.Written = written
			rec.Total = total
		}
		a.mu.Unlock()
	})
	a.mu.Lock()
	rec := a.hashes.Results[name]
	if rec == nil {
		a.mu.Unlock()
		return
	}
	if ctx.Err() != nil {
		if rec.Status == hash.StatusRunning {
			if prev != nil {
				a.hashes.Results[name] = prev
			} else {
				delete(a.hashes.Results, name)
			}
		}
		slog.Info("verify cancelled", "name", name)
		a.mu.Unlock()
		return
	}
	rec.Written = rec.Total
	rec.Checked = time.Now()
	rec.Expected = expected
	if err != nil {
		if errors.Is(err, hash.ErrMismatch) {
			rec.Status = hash.StatusMismatch
			rec.Actual = actual
			rec.Error = err.Error()
			slog.Warn("verify mismatch", "name", name, "expected", expected, "actual", actual)
		} else {
			rec.Status = hash.StatusError
			rec.Error = err.Error()
			slog.Warn("verify failed", "name", name, "err", err)
		}
		a.mu.Unlock()
		return
	}
	rec.Status = hash.StatusOK
	rec.Actual = actual
	rec.Error = ""
	slog.Info("verify ok", "name", name, "checksum", actual)
	a.mu.Unlock()
}

func (a *App) cancelVerifies() {
	a.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.verifyCancel))
	for _, cancel := range a.verifyCancel {
		cancels = append(cancels, cancel)
	}
	a.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		a.verifyWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		slog.Warn("verifies did not stop before shutdown timeout")
	}
}
