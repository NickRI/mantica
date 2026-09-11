package download

//go:generate go tool enumer -type=Status -json -trimprefix=Status -transform=lower

type Status int

const (
	StatusRunning Status = iota
	StatusPaused
	StatusError
	StatusDone
	StatusCancelled
)
