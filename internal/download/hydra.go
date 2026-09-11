package download

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/divyam234/hydra"
	"golang.org/x/time/rate"
)

const (
	Retries   = 3
	RetryWait = 10 * time.Second
)

func HydraOptions(userAgent string, onProgress hydra.ProgressFunc, getLimiter func() *rate.Limiter) hydra.Options {
	opts := hydra.DefaultOptions()
	opts.Split = 8
	opts.MaxConnectionsPerServer = 8
	opts.Timeout = 48 * time.Hour
	opts.ExistingFile = hydra.ExistingFileResume
	opts.UserAgent = userAgent
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
	opts.Transport = &RateTransport{Base: base, Get: getLimiter}
	return opts
}

type RateTransport struct {
	Base http.RoundTripper
	Get  func() *rate.Limiter
}

func (t *RateTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	lim := t.Get()
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
