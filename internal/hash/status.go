package hash

//go:generate go tool enumer -type=Status -json -trimprefix=Status -transform=lower

// Status is checksum verification state. Zero means unset (JSON omitempty).
type Status int

const (
	StatusOK Status = iota + 1
	StatusMismatch
	StatusError
	StatusRunning
)
