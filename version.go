package main

import "fmt"

// Set via -ldflags at build time (Nix / Makefile / CI).
var (
	version = "dev"
	commit  = "unknown"
)

func versionLine() string {
	return fmt.Sprintf("mantica %s commit=%s", version, commit)
}
