# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Cache dependency downloads separately from source so the layer is reused
# whenever go.mod/go.sum are unchanged.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the Worker binary
RUN go build -o /app/synthify-worker ./apps/worker/cmd/server

# Runner stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/synthify-worker /app/synthify-worker

# Default port for Cloud Run
EXPOSE 8081

CMD ["/app/synthify-worker"]
