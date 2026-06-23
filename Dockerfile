# syntax=docker/dockerfile:1.7

FROM golang:1.26.4-alpine AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download
COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/excedo-dns-deploy ./cmd/excedo-dns-deploy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/excedo-dns-cleanup ./cmd/excedo-dns-cleanup

FROM busybox:1.36.1-musl AS toolbox
RUN mkdir -p /toolbox/bin && \
    cp /bin/busybox /toolbox/bin/busybox && \
    ln -s busybox /toolbox/bin/sh && \
    ln -s busybox /toolbox/bin/cp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder --chmod=0555 /out/excedo-dns-deploy /usr/local/bin/excedo-dns-deploy
COPY --from=builder --chmod=0555 /out/excedo-dns-cleanup /usr/local/bin/excedo-dns-cleanup
COPY --from=toolbox --chmod=0555 /toolbox/bin /bin

USER 65532:65532
ENTRYPOINT ["/bin/sh", "-ec"]
CMD ["echo 'acme-gateway-hooks image: copy binaries from /usr/local/bin to your hooks volume'"]
