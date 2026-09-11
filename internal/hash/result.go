package hash

import (
	"errors"
	"time"
)

type Result struct {
	Expected string    `json:"expected,omitempty"`
	Actual   string    `json:"actual,omitempty"`
	Status   Status    `json:"status"`
	Error    string    `json:"error,omitempty"`
	Total    int64     `json:"total"`
	Written  int64     `json:"written"`
	Checked  time.Time `json:"checked,omitempty"`
}

type Store struct {
	Expected map[string]string  `json:"expected"`
	Results  map[string]*Result `json:"results"`
}

var (
	ErrNoChecksum      = errors.New("no checksum")
	ErrVerifyBusy      = errors.New("verification already running")
	ErrAlreadyVerified = errors.New("already verified")
	ErrMismatch        = errors.New("checksum mismatch")
)
