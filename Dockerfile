# syntax=docker/dockerfile:1.7

FROM golang:1.26.4-alpine AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download
COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/excedo-dns-deploy ./cmd/excedo-dns-deploy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/excedo-dns-cleanup ./cmd/excedo-dns-cleanup

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app -u 65532 app
COPY --from=builder /out/excedo-dns-deploy /usr/local/bin/excedo-dns-deploy
COPY --from=builder /out/excedo-dns-cleanup /usr/local/bin/excedo-dns-cleanup
RUN chmod 0555 /usr/local/bin/excedo-dns-deploy /usr/local/bin/excedo-dns-cleanup

USER 65532:65532
ENTRYPOINT ["/bin/sh", "-ec"]
CMD ["echo 'acme-gateway-hooks image: copy binaries from /usr/local/bin to your hooks volume'"]
