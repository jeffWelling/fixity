# Build stage
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fixity ./cmd/fixity

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/fixity .

# Create directory for monitoring test data
RUN mkdir -p /data

# Expose HTTP port
EXPOSE 8080

# Run as non-root user
RUN addgroup -g 1000 fixity && \
    adduser -D -u 1000 -G fixity fixity && \
    chown -R fixity:fixity /app /data

USER fixity

ENTRYPOINT ["/app/fixity"]
CMD ["serve"]
