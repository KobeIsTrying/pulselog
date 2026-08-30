# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY services/query-api ./services/query-api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/query-api ./services/query-api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -H -u 65532 app
COPY --from=build /out/query-api /usr/local/bin/query-api
USER 65532:65532
EXPOSE 8082
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8082/readyz || exit 1
ENTRYPOINT ["/usr/local/bin/query-api"]
