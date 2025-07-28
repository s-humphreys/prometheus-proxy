FROM golang:1.24-bullseye AS base

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

FROM base AS test
RUN go test -v ./...

FROM test AS builder
RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build \
    -a \
    -o /prometheus-proxy \
    ./cmd/main.go

FROM gcr.io/distroless/static-debian11

COPY --from=builder /prometheus-proxy /prometheus-proxy

ENTRYPOINT ["/prometheus-proxy", "run"]
