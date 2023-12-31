# Woodpecker CI Configuration Service for Nix Flakes

[![status-badge](https://build.lounge.rocks/api/badges/10/status.svg)](https://build.lounge.rocks/repos/10)

This service dynamically generates pipelines for [Woodpecker CI](https://woodpecker-ci.org/) from
Nix flakes. This allows to omit `.woodpecker.yml` files for repositories
containing a `flake.nix` file. The pipeline will the be read form a dedicated
flake output.

Woodpecker will fall back to normal `.woodpecker.yml` pipelines, if no flake is
found. There is also an option to specify a filter to limit which repositories
will be searched for `flake.nix` files for CI steps.

This works using Woodpecker's [External Configuration
API](https://woodpecker-ci.org/docs/administration/external-configuration-api).
The code is based on the official Woodpecker
[example-config-service](https://github.com/woodpecker-ci/example-config-service).

## Configuration

The service is configured via environment variables and will look for a `.env`
file at startup. The following options are available:

| Variable                  | Example               | Description                              |
|---------------------------|-----------------------|------------------------------------------|
| PIPELINER_PUBLIC_KEY_FILE | `/path/to/key.txt`    | Path to key for signature verification   |
| PIPELINER_HOST            | `localhost:8080`      | Where the service should listen          |
| PIPELINER_OVERRIDE_FILTER | `test-*`              | Regex to filter repos                    |
| PIPELINER_SKIP_VERIFY     | `true`                | Don't verify the signature.              |
| PIPELINER_FLAKE_OUTPUT    | `woodpecker-pipeline` | flake output containing the pipeline     |
| PIPELINER_PRECMDS         | `git -v`              | commands to run before building pipeline |
| PIPELINER_DEBUG           | `true`                | Debug mode, more output                  |

The public key used for verification can be retrieved from the woodpecker server
at `http(s)://your-woodpecker-server/api/signature/public-key`. An example
`.env.sample` is included, which can be copied to `.env` as a starting point.

### NixOS Service

There is also a Nix module in the flake to allow easy development on NixOS. An
example configuration could look like this after adding it to your flake inputs:

```nix
imports = [
  flake-pipeliner.nixosModules.flake-pipeliner
];

services.flake-pipeliner = {
  enable = true;
  environment = {
    PIPELINER_PUBLIC_KEY_FILE = "${./woodpecker-public-key}";
    PIPELINER_HOST = "localhost:8585";
    PIPELINER_OVERRIDE_FILTER = "test-*";
    PIPELINER_SKIP_VERIFY = "false";
    PIPELINER_FLAKE_OUTPUT = "woodpecker-pipeline";
    PIPELINER_PRECMDS = "git -v";
    PIPELINER_DEBUG = "false";
    NIX_REMOTE = "daemon";
    PAGER = "cat";
  };
};
```

## Woodpecker CI Server

The server has to be configured to use the configuration service by setting the
endpoint as shown below. See [official
documentation](https://woodpecker-ci.org/docs/administration/external-configuration-api)
for more information.

```
WOODPECKER_CONFIG_SERVICE_ENDPOINT=https://config-service-host.tld
```

Woodpecker will `POST` to the endpoint when a build is triggered (e.g. by
pushing) and submit the build metadata. The configuration service should reply
with pipeline steps. It will return `HTTP 204` to tell the server to use
existing configuration, e.g. when no `flake.nix` is found.

# Troubleshooting and Development

To test, it can be useful to mock requests to the service with curl. An example
is in included in `test-request.json` and can be submitted using:

```sh
curl -X POST -H "Content-Type: application/json" -d @test-request.json 127.0.0.1:8000
```

To test, that the server is `POST`ing correctly it can be helpful to set
`WOODPECKER_PIPELINER_ENDPOINT` to a request bin like
https://public.requestbin.com and analyze the submitted JSON
