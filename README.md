<div align="center">

  <p>
    <img src="logo.svg" width="220" alt="Mantica">
  </p>

  <h1>Mantica</h1>

  <p>
    <strong>Self-hosted MBTiles / PMTiles atlas with MapLibre UI</strong>
  </p>

  <h3>
    <a href="#nixos-flake">NixOS</a>
    <span> | </span>
    <a href="#downloads">Downloads</a>
    <span> | </span>
    <a href="#run-locally">Run locally</a>
    <span> | </span>
    <a href="#options-nixos">Options</a>
  </h3>

  <p>
    <a href="https://github.com/NickRI/mantica"><img src="https://img.shields.io/badge/GitHub-NickRI%2Fmantica-black" alt="GitHub"></a>
    <a href="https://github.com/NickRI/mantica/releases/latest"><img src="https://img.shields.io/github/v/release/NickRI/mantica" alt="Latest release"></a>
  </p>

</div>

## Introduction

**Mantica** (Latin *mantica* — a traveler’s bag): maps you keep with you on your own host.

Serves local `*.mbtiles` / `*.pmtiles`, a MapLibre front-end, catalog downloads, and optional geocoding proxies. HTTP Basic auth is optional (both credentials together, or none).

Default listen: `127.0.0.1:8091` (NixOS module). CLI default is `:8080`.

- `GET /health` — liveness (no auth)
- UI at `/`
- Tile / map APIs under `/api/*`, `/services`, `/pmtiles/`

## NixOS (flake)

```nix
{
  inputs.mantica.url = "github:NickRI/mantica";
  inputs.mantica.inputs.nixpkgs.follows = "nixpkgs";

  # modules = [ inputs.mantica.nixosModules.default ];
}
```

```nix
{
  services.mantica.enable = true;
  services.mantica.workDir = "/var/lib/mantica";
  # Optional Basic auth (both or neither):
  # services.mantica.authUserFile = config.sops.secrets."mantica/username".path;
  # services.mantica.authPassFile = config.sops.secrets."mantica/password".path;
}
```

## Downloads

Latest binary: **[GitHub Releases](https://github.com/NickRI/mantica/releases/latest)**

| Asset | Download |
|-------|----------|
| Linux amd64 binary | [`mantica-linux-amd64.tar.gz`](https://github.com/NickRI/mantica/releases/latest/download/mantica-linux-amd64.tar.gz) |

```sh
curl -fsSL -o mantica-linux-amd64.tar.gz \
  https://github.com/NickRI/mantica/releases/latest/download/mantica-linux-amd64.tar.gz
tar -xzf mantica-linux-amd64.tar.gz   # → ./mantica
```

## Run locally

```sh
go run . -dir . -listen :8080
# with auth:
go run . -dir . -auth-user USER -auth-pass PASS
```

```sh
nix build
./result/bin/mantica -dir .
```

`-dir` is the data root (default `.`):

| Path | Purpose |
|------|---------|
| `<dir>/tilesets/` | `*.mbtiles` / `*.pmtiles` |
| `<dir>/settings.json` | UI settings |
| `<dir>/downloads.json` | Download jobs |
| `<dir>/hashes.json` | Checksum verify state |
| `<dir>/geocode-cache.gz` | Geocode LRU |
| `<dir>/downloads/` | In-progress download workdirs |

Keep runtime state out of git.

## Options (NixOS)

| Option | Default | Description |
|--------|---------|-------------|
| `services.mantica.enable` | — | Enable service |
| `services.mantica.listenAddress` | `127.0.0.1` | Bind address |
| `services.mantica.port` | `8091` | Listen port |
| `services.mantica.workDir` | `/var/lib/mantica` | Data root (`-dir`) |
| `services.mantica.authUserFile` | `null` | Basic auth user file (with `authPassFile`) |
| `services.mantica.authPassFile` | `null` | Basic auth password file |
| `services.mantica.geocoderKeysFile` | `null` | JSON API keys for geocoders |
| `services.mantica.geocodeCache` | `32MB` | Geocode LRU size (`0` disables) |

## Develop

```sh
go build -o mantica .
make release-artifacts
nix build
```
