# Woodpecker CI Configuration Service for Nix Flakes

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

| Variable                       | Example               | Description                            |
|--------------------------------|-----------------------|----------------------------------------|
| CONFIG_SERVICE_PUBLIC_KEY_FILE | `/path/to/key.txt`    | Path to key for signature verification |
| CONFIG_SERVICE_HOST            | `localhost:8080`      | Where the service should listen        |
| CONFIG_SERVICE_OVERRIDE_FILTER | `test-*`              | Regex to filter repos                  |
| CONFIG_SERVICE_SKIP_VERIFY     | `true`                | Don't verify the signature.            |
| CONFIG_SERVICE_FLAKE_OUTPUT    | `woodpecker-pipeline` | flake output containing the pipeline   |

The public key used for verification can be retrieved from the woodpecker server
at `http(s)://your-woodpecker-server/api/signature/public-key`. An example
`.env.sample` is included, which can be copied to `.env` as a starting point.


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

# Toubleshooting and Development

To test, it can be useful to mock requests to the service with curl. An example
is in included in `test-request.json` and can be submitted using:

```sh
curl -X POST -H "Content-Type: application/json" -d @test-request.json 127.0.0.1:8000
```

To test, that the server is `POST`ing correctly it can be helpful to set
`WOODPECKER_CONFIG_SERVICE_ENDPOINT` to a request bin like
https://public.requestbin.com and analyze the submitted JSON
