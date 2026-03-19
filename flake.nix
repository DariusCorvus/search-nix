{
  description = "search-nix — search NixOS packages from the terminal";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "search-nix";
          version = "0.1.0";
          src = ./.;
          # Run `nix build` once to get the correct hash from the error message
          vendorHash = "sha256-GlwDnvLCTNLID0SW1wHh4f5O1t0sz+NX/k4RdC1jKk0=";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
          ];
        };
      });
}
