{
  description = "Woodpecker CI external configuration service for Nix Flakes";

  # Nixpkgs / NixOS version to use.
  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let

      # System types to support.
      supportedSystems =
        [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];

      # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system:
        import nixpkgs {
          inherit system;
          overlays = [ self.overlays.default ];
        });
    in
    {

      # A Nixpkgs overlay.
      overlays.default = final: prev: {
        flake-pipeliner = with final;
          buildGoModule {

            pname = "flake-pipeliner";
            version = "v1.0.0";
            src = ./.;
            vendorSha256 = "sha256-+JKGn+JlkX70M44ZQAe2yRdA7pFCZe+V0J35ZQFqTD0=";

            meta = with lib; {
              description = "Woodpecker CI external configuration service for Nix Flakes";
              homepage = "https://github.com/pinpox/woodpecker-flake-pipeliner";
              license = licenses.gpl3;
              mainProgram = "woodpecker-flake-pipeliner";
              maintainers = with maintainers; [ pinpox ];
            };
          };
      };

      # Package
      packages = forAllSystems (system: {
        inherit (nixpkgsFor.${system}) flake-pipeliner;
        default = self.packages.${system}.flake-pipeliner;
      });

      # Nixos module
      nixosModules.flake-pipeliner = { pkgs, lib, config, ... }:
        with lib;
        let cfg = config.services.flake-pipeliner;
        in {

          meta.maintainers = with lib.maintainers; [ pinpox ];

          options.services.flake-pipeliner = {

            package = lib.mkPackageOptionMD pkgs "flake-pipeliner" { };

            enable = lib.mkEnableOption (lib.mdDoc description);

            extraGroups = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              example = [ "podman" ];
              description = lib.mdDoc ''
                Additional groups for the systemd service.
              '';
            };

            environment = lib.mkOption {
              default = { };
              type = lib.types.attrsOf lib.types.str;
              example = lib.literalExpression
                ''
                  {
                    CONFIG_SERVICE_PUBLIC_KEY_FILE = "/path/to/key.txt";
                    CONFIG_SERVICE_HOST = "localhost:8080";
                    CONFIG_SERVICE_OVERRIDE_FILTER = "test-*";
                    CONFIG_SERVICE_SKIP_VERIFY = "true";
                    CONFIG_SERVICE_FLAKE_OUTPUT = "woodpecker-pipeline";
                  }
                '';
              description = lib.mdDoc "woodpecker-flake-pipeliner config environment variables, for other options read the [documentation](https://github.com/pinpox/woodpecker-flake-pipeliner/blob/main/README.md)";
            };
          };

          config = lib.mkIf cfg.enable {

            nixpkgs.overlays = [ self.overlays.default ];

            # Allow user to run nix
            nix.settings.allowed-users = [ "flake-pipeliner" ];

            # Service
            systemd.services.flake-pipeliner = {

              environment = cfg.environment // {
                HOME = "/run/flake-pipeliner";
              };

              path = with pkgs; [
                bash
                coreutils
                git
                git-lfs
                gnutar
                gzip
                nix
              ];

              description = "Woodpecker Flake Pipeliner Configuration Service";
              wantedBy = [ "multi-user.target" ];
              after = [ "network-online.target" ];
              wants = [ "network-online.target" ];




              serviceConfig = {

                RuntimeDirectory = "flake-pipeliner";
                # RuntimeDirectoryMode = "0700";

                # WorkingDirectory = "/var/lib/flake-pipeliner";

                User = "flake-pipeliner";
                # Group = "flake-pipeliner";
                DynamicUser = true;
                SupplementaryGroups = cfg.extraGroups;
                ExecStart = lib.getExe cfg.package;
                Restart = "on-failure";
                RestartSec = 15;
                CapabilityBoundingSet = "";
                NoNewPrivileges = true;
                ProtectSystem = "strict";
                PrivateTmp = true;
                PrivateDevices = true;
                PrivateUsers = true;
                ProtectHostname = true;
                ProtectClock = true;
                ProtectKernelTunables = true;
                ProtectKernelModules = true;
                ProtectKernelLogs = true;
                ProtectControlGroups = true;
                RestrictAddressFamilies = [ "AF_UNIX AF_INET AF_INET6" ];
                LockPersonality = true;
                MemoryDenyWriteExecute = true;
                RestrictRealtime = true;
                RestrictSUIDSGID = true;
                PrivateMounts = true;
                SystemCallArchitectures = "native";
                SystemCallFilter = "~@clock @privileged @cpu-emulation @debug @keyring @module @mount @obsolete @raw-io @reboot @swap";
                BindPaths = [
                  "/nix/var/nix/daemon-socket/socket"
                  "/run/nscd/socket"
                ];
                BindReadOnlyPaths = [
                  "${config.environment.etc."ssh/ssh_known_hosts".source}:/etc/ssh/ssh_known_hosts"
                  "-/etc/hosts"
                  "-/etc/localtime"
                  "-/etc/nsswitch.conf"
                  "-/etc/resolv.conf"
                  "-/etc/ssl/certs"
                  "-/etc/static/ssl/certs"
                  "/etc/group:/etc/group"
                  "/etc/machine-id"
                  "/etc/nix:/etc/nix"
                  "/etc/passwd:/etc/passwd"
                  # channels are dynamic paths in the nix store, therefore we need to bind mount the whole thing
                  "/nix/"
                ];
              };
            };
          };
        };
    };
}
