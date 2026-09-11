{
  description = "Mantica — self-hosted MBTiles/PMTiles atlas with MapLibre";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;

      commit = self.shortRev or self.dirtyShortRev or "unknown";

      mkMantica =
        pkgs:
        (pkgs.callPackage ./default.nix { }).overrideAttrs (old: {
          ldflags = (old.ldflags or [ ]) ++ [
            "-X main.commit=${commit}"
          ];
        });
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          mantica = mkMantica pkgs;
        in
        {
          default = mantica;
          inherit mantica;
        }
      );

      nixosModules.default =
        { pkgs, ... }:
        {
          imports = [ ./module.nix ];
          services.mantica.package = nixpkgs.lib.mkDefault self.packages.${pkgs.system}.mantica;
        };
      nixosModules.mantica = self.nixosModules.default;

      overlays.default = final: prev: {
        mantica = mkMantica final;
      };
    };
}
