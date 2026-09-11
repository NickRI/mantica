package hash

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"
	"strings"
	"time"

	"github.com/divyam234/hydra"
)

func FileProgress(ctx context.Context, path, expected string, progress func(written, total int64)) (string, error) {
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
		return actual, ErrMismatch
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
