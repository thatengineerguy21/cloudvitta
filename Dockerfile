# Stage 1: Build Frontend SPA Assets
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Static Binaries
FROM golang:1.26.5-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy compiled web dist so embed.FS packages it into /api binary
COPY --from=web-builder /app/web/dist ./internal/transport/spa/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/ingest ./cmd/ingest

# Stage 3: Minimal Distroless Runtime
# We use Google's minimal distroless image for reduced attack surface and minimal container size
FROM gcr.io/distroless/static-debian12
COPY --from=go-builder /app/api /api
COPY --from=go-builder /app/ingest /ingest
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
