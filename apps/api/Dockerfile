# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Cache dependency downloads separately from source so the layer is reused
# whenever go.mod/go.sum are unchanged.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the API binary
RUN go build -o /app/synthify-api ./apps/api/cmd/server

# Runner stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/synthify-api /app/synthify-api

# Default port for Cloud Run
EXPOSE 8080

CMD ["/app/synthify-api"]
