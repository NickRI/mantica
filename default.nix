{ pkgs }:

pkgs.buildGoModule rec {
  pname = "mantica";
  # Bump by hand for releases.
  version = "0.1.7";

  src = ./.;

  vendorHash = "sha256-DL8Thx9O3AwutPFGSQzYaMs5tWIEOA3LSSI7bTYgbjE=";

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
