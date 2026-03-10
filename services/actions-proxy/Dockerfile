FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /actions-proxy ./cmd/actions-proxy

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /actions-proxy /usr/local/bin/actions-proxy
# posd binary must be mounted or installed separately
EXPOSE 8090
ENTRYPOINT ["actions-proxy"]
