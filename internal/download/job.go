package download

import (
	"errors"
	"log/slog"
	"time"
)

type Download struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Name       string    `json:"name"`
	RemoteName string    `json:"remote_name"`
	Checksum   string    `json:"checksum,omitempty"`
	Total      int64     `json:"total"`
	Written    int64     `json:"written"`
	Speed      float64   `json:"speed"` // bytes/sec, instantaneous
	Status     Status    `json:"status"`
	Error      string    `json:"error,omitempty"`
	Created    time.Time `json:"created"`
}

var (
	ErrAlreadyDownloaded = errors.New("already downloaded")
	ErrJobNotFound       = errors.New("download not found")
	ErrResumeNotAllowed  = errors.New("download cannot be resumed")
	ErrPauseNotAllowed   = errors.New("download cannot be paused")
)

func Fail(job *Download, stage string, err error) {
	slog.Warn("download failed", "id", job.ID, "name", job.Name, "stage", stage, "err", err)
	job.Status = StatusError
	job.Error = err.Error()
}
