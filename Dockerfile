FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/dingbot ./cmd/dingbot

FROM alpine:3.20
WORKDIR /app
RUN adduser -D appuser
COPY --from=build /out/dingbot /app/dingbot
COPY config.yaml /app/config.yaml
USER appuser
ENTRYPOINT ["/app/dingbot", "-config", "/app/config.yaml"]
