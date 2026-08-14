# Build Stage
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

# Copy go mod and sum files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binaries statically
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/ingest ./cmd/ingest

# Runtime Stage
# We use Google's minimal distroless image for a dramatically smaller size and reduced attack surface
FROM gcr.io/distroless/static-debian12

WORKDIR /

# Copy the compiled binaries from the builder stage
COPY --from=builder /app/api /api
COPY --from=builder /app/ingest /ingest

# Provide a default port for Cloud Run
ENV PORT=8080
EXPOSE 8080

# Run the API by default
USER nonroot:nonroot
ENTRYPOINT ["/api"]
