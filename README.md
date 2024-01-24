# Single Page Applications Base Image

The Single Page Applications Base Image is a preconfigured image and service tailored for single page applications. It functions as a straightforward static resource server with optional fallback to `index.html` if a resource is not found. It also supports Brotli and Gzip precompressed files and allows for customization of response headers. Additionally, it comes with built-in instrumentation for [OpenTelemetry](https://opentelemetry.io/) and [Prometheus](https://prometheus.io/).

## Usage

To use this base image, follow these steps:

1. Create a Dockerfile based on `ghcr.io/polyfea/spa_base`.
2. Copy your static files to the `/spa/public` directory within your Docker image.
3. The image will automatically serve your static files on port `7105`.

This makes it easy to deploy and serve your single page application with minimal configuration.

```Dockerfile
FROM ghcr.io/polyfea/spa_base:latest

COPY ./build /spa/public

ENV OTEL_SERVICE_NAME=my_spa_server
```

Build and start the container

```bash
docker build -t my_spa_server .
docker run -p 7105:7105 my_spa_server
```

## Configuration

To configure the Single Page Applications Base Image, you'll need to modify the `/spa/config/spa-base.yaml` configuration file or change environment variables. Below are the available options:

```yaml
# Port to Listen On (Default: 7105)
# Specify the port number for the server to listen on. The default port is 7105.
port: 7105

# Root Directories (Default: /spa/public)
# Define an array of root directories to search for static files. By default,
# it looks in the /spa/public directory.
roots: 
- /spa/public

# Base URL (Default: /)
# Specify the base URL for the server. The request's path must be prefixed with
# this value. The remaining path is then searched relatively to the roots
# folders. 
base-url: /

# Disable Fallback to index.html (Default: false)
# Setting this option to true will disable the fallback behavior to index.html
# for all paths.
fallback-disabled: false

# Regular Expressions for No Fallback Paths (Default: empty)
# Specify an array of regular expressions to match paths that should not
# fallback to index.html. By default, the server falls back to index.html
# for paths with a request header of Accept containing text/html or if
# Accept is not present. You can completely disable this behavior by
# setting fallback-disabled to true or by providing regular expressions
# that match specific paths.
no-fallback-regexp: []

# Response Headers to Add to All OK Responses (Default: empty)
# You can specify a set of response headers to be included in all successful
# (OK) responses. By default, this section is empty.
# 
# Default behaviour is to add `Cache-Control: no-cache` header to index.html responses, 
# and `Cache-Control: public, max-age=31536000, immutable` to all other responses.
# 
# Example:
# headers:
#   "X-Frame-Options": "DENY",
#   "X-XSS-Protection": "1; mode=block",
headers: {}

# Response Headers to Add to OK Responses Matching Regular Expressions (Default: empty)
# Define response headers that should be included in OK responses only when the
# request path matches a specific regular expression. By default, this section is empty.
# 
# Example:
# headers-per-regexp:
#   "^.*\\.json$":
#     "Cache-Control": "no-cache, no-store, must-revalidate"
headers-per-regexp: {}

# Disable Brotli Compression (Default: false)
# By default, resources are provided in Brotli-encoded format if there is a
# file with the same name and a .br extension. Set this option to true to 
# disable Brotli compression.
#
# To generate the required Brotli files, you can use the following tooling:
# Install 'preprocess' with 'npm i -D preprocess'.
brotli-disabled: false

# Disable Gzip Compression (Default: false)
# By default, resources are provided in Gzip-encoded format if there is a
# file with the same name and a .gz extension. Set this option to true to 
# disable Gzip compression.
#
# To generate the required Gzip files, you can use the following tooling:
# Install 'preprocess' with 'npm i -D preprocess'.
gzip-disabled: false

# Logging Level (Default: info)
# Specify the desired logging level, which can be one of the following: debug, info, warn, error. 
# The default level is set to 'info'.
logging-level: info

# Provide JSON Logs (Default: false)
# Enabling this option will output logs in JSON format. By default, it is disabled.
json-logging: false

# Disable OpenTelemetry Exporters Initialization (Default: false)
# When set to true, this option disables the initialization of OpenTelemetry exporters. 
# The default behavior is to initialize them using noop exporters.
telemetry-disabled: false
```

## Environment Variables

You can use the following environment variables to override the configuration file:

| Environment Variable             | Default    | Description                                                   |
| -------------------------------- | ---------- | ------------------------------------------------------------- |
| SPA_BASE_PORT                    | 7105       | Port to listen
on                                             |
| SPA_BASE_BASE_URL                | /       | Base URL for the server. The request's path must be prefixed with this value. The remaining path is then searched relatively to the `ROOTS` directory |
| SPA_BASE_ROOTS                   | /spa/public | Path to the static files                                      |
| SPA_BASE_ALLOW_SKIP_BASE_URL | false | If enabled then requests not matching base URL prefix will be processed as if the base url is set to `/`. This enables same processing with base url prefix stripped or remaining on the request path |
| SPA_BASE_FALLBACK_DISABLED       | false      | Disables fallbacks to index.html                             |
| SPA_BASE_BROTLI_DISABLED         | false      | Disables Brotli compression                                   |
| SPA_BASE_GZIP_DISABLED           | false      | Disables Gzip compression                                     |
| SPA_BASE_LOGGING_LEVEL           | info       | Logging level (debug, info, warn, error)                      |
| SPA_BASE_JSON_LOGGING            | false      | Provide JSON logs                                            |
| SPA_BASE_TELEMETRY_DISABLED      | false      | Disable OpenTelemetry exporters initialization                |
| OTEL_TRACES_EXPORTER             | none       | Tracing exporter options (none, otlp, prometheus, console). See [NewSpanExporter](https://pkg.go.dev/go.opentelemetry.io/contrib/exporters/autoexport#NewSpanExporter) and [Open Telemetry Environment Variables](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) documentation for details. |
| OTEL_METRICS_EXPORTER            | none       | Metrics exporter options (none, otlp, prometheus, console). See [NewMetricsExporter](https://pkg.go.dev/go.opentelemetry.io/contrib/exporters/autoexport#NewMetricReader) and [Open Telemetry Environment Variables](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) documentation for details. |
| OTEL_SERVICE_NAME                | spa_base   | Resource (this) service name - override to distinguish your service in telemetry results. |
