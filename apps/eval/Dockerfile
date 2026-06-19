# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Cache dependency downloads separately from source so the layer is reused
# whenever go.mod/go.sum are unchanged.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/synthify-eval ./apps/eval/cmd

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/synthify-eval /app/synthify-eval
COPY apps/eval/cases /app/apps/eval/cases
COPY apps/eval/testdata /app/apps/eval/testdata

ENTRYPOINT ["/app/synthify-eval"]
CMD ["--cases", "apps/eval/cases", "--format", "json"]
