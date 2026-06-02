FROM golang:1.26 AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o /out/vexod ./cmd/vexod

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="vexo-consensus"
LABEL org.opencontainers.image.description="Experimental modular consensus engine skeleton"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"

COPY --from=builder /out/vexod /usr/local/bin/vexod
WORKDIR /var/lib/vexo
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/vexod"]
CMD ["status"]
