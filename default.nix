{ pkgs, ... }:

pkgs.buildGoModule {
  pname = "mantica";
  version = "0.1.1";

  src = ./.;

  vendorHash = "sha256-7YMB0HXqAPuyzwuEKQlQ1RcSkpzAtiTn2GZaWpGnx98=";

  doCheck = false;

  meta = with pkgs.lib; {
    description = "Self-hosted MBTiles/PMTiles map server with MapLibre UI";
    homepage = "https://github.com/NickRI/mantica";
    license = licenses.mit;
    mainProgram = "mantica";
    platforms = platforms.linux;
  };
}
