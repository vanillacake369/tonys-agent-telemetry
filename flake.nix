{
  description = "TUI dashboard for Claude Code sessions, agents, DAG visualization, and skill marketplace";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = "0.1.0";
      in {
        packages = {
          default = pkgs.buildGoModule {
            pname = "tonys-agent-telemetry";
            inherit version;
            src = ./.;
            vendorHash = "sha256-abS9Oz46kVFZk63vEm8fAVb/zMhwsMAKbdioU6FRVRw=";

            subPackages = [ "." ];

            ldflags = [
              "-s" "-w"
              "-X main.version=${version}"
            ];

            nativeBuildInputs = [ pkgs.makeWrapper ];

            postInstall = ''
              # Wrap main binary with optional gh in PATH
              wrapProgram $out/bin/tonys-agent-telemetry \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}
            '';

            meta = with pkgs.lib; {
              description = "TUI dashboard for Claude Code";
              license = licenses.mit;
              mainProgram = "tonys-agent-telemetry";
              platforms = platforms.unix;
            };
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls gotools gh ];
        };
      }
    );
}
