{ forCI ? false }: let
  pkgs = import <nixpkgs> {};
in
  with pkgs;
  mkShell {
    buildInputs = [
      go
      imagemagick
      olm
      pre-commit
    ] ++ lib.lists.optional (!forCI) [
      gotools
      gopls
      vgo2nix
      yq
    ];
  }
