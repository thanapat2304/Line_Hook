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