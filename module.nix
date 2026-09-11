{
  config,
  pkgs,
  lib,
  ...
}:

let
  manticaPkg = pkgs.callPackage ./default.nix { };
  cfg = config.services.mantica;
  authEnabled = cfg.authUserFile != null;
in
{
  imports = [
    (lib.mkRenamedOptionModule
      [
        "services"
        "mantica"
        "tilesDir"
      ]
      [
        "services"
        "mantica"
        "workDir"
      ]
    )
  ];

  options.services.mantica = {
    enable = lib.mkEnableOption "Mantica MBTiles/PMTiles map server with MapLibre UI";

    package = lib.mkOption {
      type = lib.types.package;
      default = manticaPkg;
      description = "mantica package.";
    };

    user = lib.mkOption {
      type = lib.types.str;
      default = "mantica";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "mantica";
    };

    listenAddress = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 8091;
    };

    workDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/mantica";
      description = "Data root passed as -dir. Contains tilesets/, settings.json, downloads/, caches.";
    };

    authUserFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = "File with HTTP Basic username. Must be set together with authPassFile, or both left null (no auth).";
    };

    authPassFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = "File with HTTP Basic password. Must be set together with authUserFile, or both left null (no auth).";
    };

    geocoderKeysFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = "JSON file with geocoder API keys: { \"nominatim\": \"…\", \"photon\": \"…\" }.";
    };

    geocodeCache = lib.mkOption {
      type = lib.types.str;
      default = "32MB";
      description = "Geocode LRU cache size (human-readable, e.g. 32MB, 1GiB; 0 disables). Stored as <workDir>/geocode-cache.gz.";
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = (cfg.authUserFile == null) == (cfg.authPassFile == null);
        message = "services.mantica.authUserFile and authPassFile must both be set or both null";
      }
    ];

    users.users = lib.optionalAttrs (cfg.user == "mantica") {
      mantica = {
        isSystemUser = true;
        group = cfg.group;
        home = cfg.workDir;
      };
    };

    users.groups = lib.optionalAttrs (cfg.group == "mantica") {
      mantica = { };
    };

    systemd.tmpfiles.rules = [
      "d ${cfg.workDir} 0755 ${cfg.user} ${cfg.group} -"
      "d ${cfg.workDir}/tilesets 0755 ${cfg.user} ${cfg.group} -"
      "d ${cfg.workDir}/downloads 0755 ${cfg.user} ${cfg.group} -"
    ];

    systemd.services.mantica = {
      description = "Mantica MBTiles/PMTiles map server";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        User = cfg.user;
        Group = cfg.group;
        Restart = "on-failure";
        RestartSec = "5s";
        WorkingDirectory = cfg.workDir;
        TimeoutStopSec = "60s";
        KillSignal = "SIGTERM";
      };

      script = ''
        ${lib.optionalString authEnabled ''
          auth_user=$(cat ${cfg.authUserFile})
          auth_pass=$(cat ${cfg.authPassFile})
        ''}
        exec ${cfg.package}/bin/mantica \
          -listen ${cfg.listenAddress}:${toString cfg.port} \
          -dir ${cfg.workDir} \
          ${lib.optionalString authEnabled ''-auth-user "$auth_user" -auth-pass "$auth_pass"''} \
          ${lib.optionalString (cfg.geocoderKeysFile != null) "-geocoder-keys ${cfg.geocoderKeysFile}"} \
          -geocode-cache ${lib.escapeShellArg cfg.geocodeCache}
      '';

      restartTriggers = [
        cfg.listenAddress
        (toString cfg.port)
        cfg.workDir
        cfg.geocodeCache
      ]
      ++ lib.optional authEnabled cfg.authUserFile
      ++ lib.optional authEnabled cfg.authPassFile
      ++ lib.optional (cfg.geocoderKeysFile != null) cfg.geocoderKeysFile;
    };
  };
}
