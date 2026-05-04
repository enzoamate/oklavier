FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/oklavier-api ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl bash && \
    curl -fsSL -o /tmp/helm.tar.gz https://get.helm.sh/helm-v3.16.1-linux-amd64.tar.gz && \
    tar -xzf /tmp/helm.tar.gz -C /tmp && \
    mv /tmp/linux-amd64/helm /usr/local/bin/helm && \
    rm -rf /tmp/helm.tar.gz /tmp/linux-amd64 && \
    curl -fsSL -o /usr/local/bin/kubectl https://dl.k8s.io/release/v1.31.0/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl
COPY --from=builder /out/oklavier-api /usr/local/bin/oklavier-api
EXPOSE 8080
CMD ["oklavier-api"]
