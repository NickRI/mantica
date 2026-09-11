{ pkgs }:

pkgs.buildGoModule rec {
  pname = "mantica";
  # Bump by hand for releases.
  version = "0.1.6";

  src = ./.;

  vendorHash = "sha256-7YMB0HXqAPuyzwuEKQlQ1RcSkpzAtiTn2GZaWpGnx98=";

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  doCheck = false;

  meta = with pkgs.lib; {
    description = "Self-hosted MBTiles/PMTiles map server with MapLibre UI";
    homepage = "https://github.com/NickRI/mantica";
    license = licenses.mit;
    mainProgram = "mantica";
    platforms = platforms.linux;
  };
}
