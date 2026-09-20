# ---- Build stage ----
# Compile the binary in a full Go image.
FROM golang:1.27-alpine AS build

WORKDIR /src

# Download dependencies first so Docker can cache this layer
# and skip it when only source code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 builds a static binary that runs without C libraries.
# -ldflags="-s -w" strips debug info to make the binary smaller.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /healthcheck .

# ---- Run stage ----
# distroless/static has only CA certificates (needed for https) and timezone
# data: no shell or package manager, so the image is tiny and more secure.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /healthcheck /app/healthcheck
COPY services.yaml /app/services.yaml

# Run as an unprivileged user.
USER nonroot:nonroot

# The dashboard port (only used when -web is passed).
EXPOSE 8080

ENTRYPOINT ["/app/healthcheck"]
# Default arguments; override them by passing your own after the image name.
CMD ["-config", "/app/services.yaml", "-interval", "30s"]
