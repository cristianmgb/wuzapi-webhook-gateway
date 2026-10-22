# syntax=docker/dockerfile:1
FROM golang:1.26.10-alpine3.23@sha256:0525c952defebc693cdaf62cdf43afa32be5a7f377e99260642ea19c240c708e AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/wuzapi-webhook-gateway \
    ./cmd/server

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/wuzapi-webhook-gateway /wuzapi-webhook-gateway

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/wuzapi-webhook-gateway"]
