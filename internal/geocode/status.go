package geocode

//go:generate go tool enumer -type=Status -json -trimprefix=Status -transform=lower

// Status is geocoder health: zero = idle (omitted in JSON), ok, fail.
type Status int

const (
	StatusOK Status = iota + 1
	StatusFail
)
