FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /burrow ./cmd/burrow

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /burrow .
COPY templates/ ./templates/

ENTRYPOINT ["./burrow", "--config", "/etc/burrow/config.yaml"]
