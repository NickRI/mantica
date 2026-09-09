package main

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/divyam234/hydra"
)

type HashResult struct {
	Expected string    `json:"expected,omitempty"`
	Actual   string    `json:"actual,omitempty"`
	Status   string    `json:"status"`
	Error    string    `json:"error,omitempty"`
	Total    int64     `json:"total"`
	Written  int64     `json:"written"`
	Checked  time.Time `json:"checked,omitempty"`
}

type hashStore struct {
	Expected map[string]string      `json:"expected"`
	Results  map[string]*HashResult `json:"results"`
}

var (
	errNoChecksum = errors.New("no checksum")
	errVerifyBusy = errors.New("verification already running")
)

func (a *App) hashesPath() string {
	return filepath.Join(a.dir, ".hashes.json")
}

func (a *App) loadHashes() {
	a.hashes.Expected = map[string]string{}
	a.hashes.Results = map[string]*HashResult{}
	data, err := os.ReadFile(a.hashesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	if err := json.Unmarshal(data, &a.hashes); err != nil {
		panic(err)
	}
	if a.hashes.Expected == nil {
		a.hashes.Expected = map[string]string{}
	}
	if a.hashes.Results == nil {
		a.hashes.Results = map[string]*HashResult{}
	}
	for name, rec := range a.hashes.Results {
		if rec != nil && rec.Status == "running" {
			delete(a.hashes.Results, name)
		}
	}
}

func (a *App) saveHashes() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.saveHashesLocked()
}

func (a *App) saveHashesLocked() {
	data, err := json.MarshalIndent(a.hashes, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(a.hashesPath(), data, 0o644); err != nil {
		panic(err)
	}
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

func (a *App) decorateHash(info *MapInfo) {
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

func (a *App) startVerify(id, kind string) (*HashResult, error) {
	if kind == "" {
		var err error
		kind, err = detectMapKind(a.dir, id)
		if err != nil {
			return nil, err
		}
	}
	path := tilesetPath(a.dir, id, kind)
	name := filepath.Base(path)
	a.mu.Lock()
	expected := a.expectedChecksumLocked(name)
	if expected == "" {
		a.mu.Unlock()
		return nil, errNoChecksum
	}
	if len(a.verifyCancel) > 0 {
		if rec := a.hashes.Results[name]; rec != nil && rec.Status == "running" {
			out := *rec
			a.mu.Unlock()
			return &out, nil
		}
		a.mu.Unlock()
		return nil, errVerifyBusy
	}
	if _, err := hydra.ParseChecksum(expected); err != nil {
		a.mu.Unlock()
		return nil, err
	}
	a.hashes.Expected[name] = expected
	prev := a.hashes.Results[name]
	rec := &HashResult{Expected: expected, Status: "running", Total: 0}
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

func (a *App) runVerify(ctx context.Context, cancel context.CancelFunc, name, path, expected string, prev *HashResult) {
	defer a.verifyWG.Done()
	defer func() {
		a.mu.Lock()
		delete(a.verifyCancel, name)
		a.mu.Unlock()
		cancel()
		a.saveHashes()
	}()

	slog.Info("verify start", "name", name, "path", path)
	actual, err := hashFileProgress(ctx, path, expected, func(written, total int64) {
		a.mu.Lock()
		if rec := a.hashes.Results[name]; rec != nil && rec.Status == "running" {
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
		if rec.Status == "running" {
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
		if errors.Is(err, errHashMismatch) {
			rec.Status = "mismatch"
			rec.Actual = actual
			rec.Error = err.Error()
			slog.Warn("verify mismatch", "name", name, "expected", expected, "actual", actual)
		} else {
			rec.Status = "error"
			rec.Error = err.Error()
			slog.Warn("verify failed", "name", name, "err", err)
		}
		a.mu.Unlock()
		return
	}
	rec.Status = "ok"
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

func (a *App) clearOrphanDownloads() int {
	root := filepath.Join(a.dir, ".downloads")
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
		case "running", "paused", "error":
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
	return removed
}

var errHashMismatch = errors.New("checksum mismatch")

func hashFileProgress(ctx context.Context, path, expected string, progress func(written, total int64)) (string, error) {
	cs, err := hydra.ParseChecksum(expected)
	if err != nil {
		return "", err
	}
	h, err := hasherFor(cs.Algorithm)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	buf := make([]byte, 1<<20)
	var written int64
	last := time.Time{}
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, err := h.Write(buf[:n]); err != nil {
				return "", err
			}
			written += int64(n)
			if progress != nil && (time.Since(last) > 200*time.Millisecond || written == st.Size()) {
				last = time.Now()
				progress(written, st.Size())
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	actual := strings.ToLower(cs.Algorithm) + ":" + hex.EncodeToString(h.Sum(nil))
	want := strings.ToLower(strings.TrimSpace(cs.Algorithm) + ":" + strings.TrimSpace(cs.Value))
	if !strings.EqualFold(actual, want) {
		return actual, errHashMismatch
	}
	return actual, nil
}

func hasherFor(alg string) (hash.Hash, error) {
	switch strings.ToLower(strings.TrimSpace(alg)) {
	case "sha256", "sha-256":
		return sha256.New(), nil
	case "sha512", "sha-512":
		return sha512.New(), nil
	case "sha1", "sha-1":
		return sha1.New(), nil
	case "md5":
		return md5.New(), nil
	default:
		return nil, errors.New("unsupported checksum algorithm: " + alg)
	}
}
