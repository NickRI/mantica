package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/NickRI/mantica/internal/app"
	"github.com/docker/go-units"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	dir := flag.String("dir", ".", "Data root: tilesets/, settings.json, downloads/, caches")
	authUser := flag.String("auth-user", "", "HTTP Basic username")
	authPass := flag.String("auth-pass", "", "HTTP Basic password")
	geocoderKeys := flag.String("geocoder-keys", "", "JSON file with geocoder API keys by id")
	geocodeCache := flag.String("geocode-cache", "32MB", "Geocode LRU cache size (e.g. 32MB, 1GiB; 0 disables)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(versionLine())
		return
	}

	if (*authUser == "") != (*authPass == "") {
		slog.Error("both -auth-user and -auth-pass are required together")
		os.Exit(1)
	}

	cacheBytes, err := units.RAMInBytes(*geocodeCache)
	if err != nil {
		slog.Error("invalid -geocode-cache", "value", *geocodeCache, "err", err)
		os.Exit(1)
	}

	err = app.Run(app.Options{
		Listen:            *listen,
		Dir:               *dir,
		AuthUser:          *authUser,
		AuthPass:          *authPass,
		GeocoderKeysPath:  *geocoderKeys,
		GeocodeCacheBytes: cacheBytes,
		Version:           version,
		Commit:            commit,
		Web:               webFS,
		Logo:              logoSVG,
		Catalog:           catalogJSON,
	})
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
