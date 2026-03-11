FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy module definition first so the layer is cached until go.mod changes.
COPY go.mod ./
RUN go mod download

# Copy source and the embedded HTML file.
COPY *.go ./
COPY index.html ./

# Build a statically linked binary with debug info stripped.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o tally .

# ── Runtime image ────────────────────────────────────────────────────────────
# Use a minimal base with a non-root user for security.
FROM alpine:3.21
RUN addgroup -S tally && adduser -S tally -G tally
WORKDIR /app
COPY --from=builder /app/tally .
USER tally
EXPOSE 9200
CMD ["./tally"]