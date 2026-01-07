# Omniphi Faucet Service
# Multi-stage build for minimal production image

FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git make gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o faucet .

# Production image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/faucet .

# Create non-root user
RUN adduser -D -u 1000 faucet
USER faucet

EXPOSE 8080

ENTRYPOINT ["./faucet"]
