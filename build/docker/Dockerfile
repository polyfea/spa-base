FROM golang AS builder

RUN addgroup --system --gid 7105 spa 
RUN adduser --system --uid 7105 --gid 7105 spa
WORKDIR $GOPATH/src/spa_base

COPY ./go.*  ./
RUN go mod download

COPY . . 

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64  \
    go build \ 
      -ldflags="-w -s" \
      -installsuffix 'static' \
      -o /go/bin/spa_d ./cmd/spa_d

# verify tests
RUN CGO_ENABLED=0 go test ./...

FROM scratch

LABEL org.opencontainers.image.authors="Polyfea Contributors"
LABEL org.opencontainers.image.url="https://github.com/polyfea/spa_base/blob/main/readme.md"
LABEL org.opencontainers.image.source="https://github.com/polyfea/spa_base.git"
LABEL org.opencontainers.image.vendor="Polyfea Team"
LABEL org.opencontainers.image.licence="Apache-2.0"
LABEL org.opencontainers.image.title="SPA_BASE"
LABEL org.opencontainers.image.description="Base image for Single Page Applications"

# PRECONFIGURED ENVIRONMENTS
ENV OTEL_METRICS_EXPORTER=none
ENV OTEL_TRACES_EXPORTER=none
ENV SPA_BASE_PORT=7105

# Override in specific instances
ENV OTEL_SERVICE_NAME=spa_base 

# Logging settings - levels are : trace, debug, information, warning, error 
ENV SCIDA_AGENT_LEVEL information
ENV SCIDA_AGENT_LOGGING false

# users
COPY --from=builder /etc/passwd /etc/passwd

USER spa

COPY --from=builder /go/bin /spa

# in your image add content to /spa/public to have something to serve
# COPY ./public /spa/public

# if you want to change configuration you can add a config.yaml file
# COPY ./config.yaml /spa/config/spa_base.yaml

# Override in service name in your image
# ENV OTEL_SERVICE_NAME=my_spa 

WORKDIR /spa

EXPOSE 7105

ENTRYPOINT ["/spa/spa_d"]
