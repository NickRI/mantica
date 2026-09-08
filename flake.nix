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
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.callPackage ./default.nix { };
          mantica = self.packages.${system}.default;
        }
      );

      nixosModules.default = import ./module.nix;
      nixosModules.mantica = self.nixosModules.default;

      overlays.default = final: prev: {
        mantica = final.callPackage ./default.nix { };
      };
    };
}
