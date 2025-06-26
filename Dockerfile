<<<<<<< HEAD
# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

WORKDIR /app

# ติดตั้ง git สำหรับ go get dependencies
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o app main.go

# ---
FROM alpine:latest
WORKDIR /app

# ติดตั้ง CA certificates สำหรับ HTTPS
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/app .
COPY config.json ./

EXPOSE 8071

CMD ["./app"] 
=======
# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates (needed for go mod download)
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main .

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/status || exit 1

# Run the application
CMD ["./main"] 
>>>>>>> b3c1019590f2da421bd51a469bdb96d55a94acc4
