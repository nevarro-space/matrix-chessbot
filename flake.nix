{
  description = "matrix-chessbot";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    (flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        ciPackages = with pkgs; [ go imagemagick olm pre-commit ];
      in rec {
        packages.matrix-chessbot = pkgs.buildGoModule {
          pname = "matrix-chessbot";
          version = "unstable-2023-12-04";
          src = self;

          propagatedBuildInputs = [ pkgs.olm ];

          vendorHash = "sha256-7D2LJIua1dSbHsS5ZT5PmdcKsZVNGE4mWxRoyGSlFeg=";
        };
        defaultPackage = packages.matrix-chessbot;

        devShells = {
          default = pkgs.mkShell {
            packages = ciPackages ++ (with pkgs; [ gotools yq ]);
          };
          ci = pkgs.mkShell { packages = ciPackages; };
        };
      }));
}
