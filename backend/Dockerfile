# syntax=docker/dockerfile:1
# Multi-stage build producing a tiny static backend image. Build context is the
# repo root (see docker-compose.yml).
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/rayancheca/bgpulse/backend/internal/config.Version=docker" \
    -o /out/bgpulse ./cmd/bgpulse

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bgpulse /bin/bgpulse
EXPOSE 8080
ENTRYPOINT ["/bin/bgpulse"]
CMD ["-mode", "demo", "-listen", ":8080", "-log-format", "json"]
