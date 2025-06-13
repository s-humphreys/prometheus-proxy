FROM golang:1.24-bullseye AS builder

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build \
      -a \
      -o /prometheus-proxy \
      ./cmd/main.go

FROM debian:bullseye-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /prometheus-proxy /prometheus-proxy

ENTRYPOINT ["/prometheus-proxy", "run"]
