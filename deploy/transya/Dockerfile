FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    librdkafka-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/microbusiness/transya.git /app

WORKDIR /app

RUN CGO_ENABLED=1 GOOS=linux go build -o transya ./cmd/main.go


FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    librdkafka1 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/transya /usr/local/bin/transya

CMD ["transya"]
