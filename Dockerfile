# Multi-stage build per GoLeapAI
# Stage 1: Build stage
FROM golang:1.25-alpine AS builder

# Installa dipendenze di build
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Imposta working directory
WORKDIR /build

# Copia go mod files
COPY go.mod go.sum ./

# Download dipendenze
RUN go mod download

# Copia codice sorgente
COPY . .

# Build binary con ottimizzazioni
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -tags="sqlite_omit_load_extension" \
    -o goleapai \
    ./cmd/backend

# Stage 2: Runtime stage
FROM alpine:latest

# Installa ca-certificates per HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Crea utente non-root
RUN addgroup -g 1000 goleapai && \
    adduser -D -u 1000 -G goleapai goleapai

# Crea directories
RUN mkdir -p /app/data /app/configs /app/logs && \
    chown -R goleapai:goleapai /app

WORKDIR /app

# Copia binary dal builder
COPY --from=builder /build/goleapai .

# Copia file di configurazione
COPY --chown=goleapai:goleapai configs/config.yaml ./configs/

# Switch to non-root user
USER goleapai

# Esponi porte
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Volume per persistenza dati
VOLUME ["/app/data", "/app/logs"]

# Entrypoint
ENTRYPOINT ["./goleapai"]
CMD ["--config", "configs/config.yaml"]
