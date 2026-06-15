FROM golang:1.26-bookworm AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apt-get update \
  && apt-get install -y --no-install-recommends build-essential ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o /out/vexod ./cmd/vexod

FROM gcr.io/distroless/base-debian12:nonroot

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="vexo-consensus"
LABEL org.opencontainers.image.description="Vexo consensus node with cgo-backed supranational/blst BLS support"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.variant="cgo"

COPY --from=builder /out/vexod /usr/local/bin/vexod
WORKDIR /var/lib/vexo
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/vexod"]
CMD ["status"]

