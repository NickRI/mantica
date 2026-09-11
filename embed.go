package main

import "embed"

// Static assets live at the repo root (go:embed cannot reach here from internal/).

//go:embed web
var webFS embed.FS

//go:embed logo.svg
var logoSVG []byte

//go:embed catalog.json
var catalogJSON []byte
