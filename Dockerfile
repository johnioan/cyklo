# Cyklo container image (x86_64 / linux/amd64).
# Build:  docker build --build-arg VERSION=0.1.8 -t cyklo .

FROM golang:1.24-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY app/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/cyklo .

FROM alpine:3.20
# tzdata: lets the TZ variable work (reminder times and "today" follow it)
RUN apk add --no-cache tzdata \
 && addgroup -S -g 10001 cyklo \
 && adduser -S -u 10001 -G cyklo cyklo \
 && mkdir /data && chown cyklo:cyklo /data
COPY --from=build /out/cyklo /usr/local/bin/cyklo

ENV CYKLO_DATA=/data \
    CYKLO_PORT=8765 \
    TZ=UTC
USER cyklo
VOLUME /data
EXPOSE 8765
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${CYKLO_PORT}/api/version" || exit 1
ENTRYPOINT ["/usr/local/bin/cyklo"]
