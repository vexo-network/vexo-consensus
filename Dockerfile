FROM golang:1.26 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/vexod ./cmd/vexod

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/vexod /usr/local/bin/vexod
WORKDIR /var/lib/vexo
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/vexod"]
CMD ["status"]
