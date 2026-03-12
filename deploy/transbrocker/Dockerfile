FROM golang:1.25-bookworm AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends librdkafka-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o transbroker ./cmd/main.go


FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends librdkafka1 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/transbroker .

EXPOSE 8080

CMD ["./transbroker"]
