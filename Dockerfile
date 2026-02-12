FROM golang:1.26.0-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o grantsy ./cmd/grantsy

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/grantsy /usr/local/bin/grantsy

USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["grantsy"]
CMD ["-config", "/etc/grantsy/config.yaml"]
