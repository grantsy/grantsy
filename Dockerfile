FROM golang:1.25.7-alpine AS builder

WORKDIR /build

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o grantsy ./cmd/grantsy

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/grantsy /usr/local/bin/grantsy

USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["grantsy"]
CMD ["-config", "/etc/grantsy/config.yaml"]
