package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/docker/go-units"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	dir := flag.String("dir", "./tilesets", "Directory with mbtiles files")
	authUser := flag.String("auth-user", "", "HTTP Basic username")
	authPass := flag.String("auth-pass", "", "HTTP Basic password")
	geocoderKeys := flag.String("geocoder-keys", "", "JSON file with geocoder API keys by id")
	geocodeCache := flag.String("geocode-cache", "32MB", "Geocode LRU cache size (e.g. 32MB, 1GiB; 0 disables)")
	flag.Parse()

	if (*authUser == "") != (*authPass == "") {
		slog.Error("both -auth-user and -auth-pass are required together")
		os.Exit(1)
	}

	cacheBytes, err := units.RAMInBytes(*geocodeCache)
	if err != nil {
		slog.Error("invalid -geocode-cache", "value", *geocodeCache, "err", err)
		os.Exit(1)
	}

	if err := run(*listen, *dir, *authUser, *authPass, *geocoderKeys, cacheBytes); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
