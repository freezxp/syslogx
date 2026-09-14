# syntax=docker/dockerfile:1.7
FROM golang:1.27.0-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend ./backend
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/syslogx ./backend/cmd/syslogx

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/syslogx /usr/local/bin/syslogx
COPY config/syslogx.yaml /etc/syslogx/syslogx.yaml
USER nonroot:nonroot
EXPOSE 8080 1514/tcp 1514/udp
ENTRYPOINT ["/usr/local/bin/syslogx"]
CMD ["-config", "/etc/syslogx/syslogx.yaml"]
