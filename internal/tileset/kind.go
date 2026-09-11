package tileset

//go:generate go tool enumer -type=Kind -json -linecomment

type Kind int

const (
	KindMBTiles Kind = iota // mbtiles
	KindPMTiles             // pmtiles
)
