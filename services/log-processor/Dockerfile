# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY services/log-processor ./services/log-processor
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/log-processor ./services/log-processor

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -H -u 65532 app
COPY --from=build /out/log-processor /usr/local/bin/log-processor
USER 65532:65532
EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8081/readyz || exit 1
ENTRYPOINT ["/usr/local/bin/log-processor"]
