FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/server .

FROM debian:bookworm-slim AS runtime

ARG TARGETARCH
ENV LIVEKIT_VERSION=1.11.0

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl tar \
    && rm -rf /var/lib/apt/lists/*

RUN case "${TARGETARCH}" in \
        amd64) LIVEKIT_ARCH="amd64" ;; \
        arm64) LIVEKIT_ARCH="arm64" ;; \
        *) echo "Unsupported TARGETARCH: ${TARGETARCH}" && exit 1 ;; \
    esac \
    && curl -fsSL "https://github.com/livekit/livekit/releases/download/v${LIVEKIT_VERSION}/livekit_${LIVEKIT_VERSION}_linux_${LIVEKIT_ARCH}.tar.gz" -o /tmp/livekit.tar.gz \
    && tar -xzf /tmp/livekit.tar.gz -C /tmp \
    && install -m 0755 /tmp/livekit-server /usr/local/bin/livekit-server \
    && rm -rf /tmp/livekit.tar.gz /tmp/livekit-server

COPY --from=builder /out/server /app/server

RUN mkdir -p /app/data

EXPOSE 8443/tcp 7880/tcp 7881/tcp 40000-40100/udp

CMD ["./server"]
