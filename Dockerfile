# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# Pull build args from Makefile
ARG PROTOC_VERSION
ARG PROTOC_GEN_GO_VERSION
ARG PROTOC_GEN_GRPC_VERSION

# Make sure we install specific version of protoc - 35.0
RUN apk add --no-cache curl unzip git && \
    mkdir -p /third_party/protoc && \
    curl -L "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip" -o /tmp/protoc.zip && \
    unzip -o /tmp/protoc.zip -d /third_party/protoc && \
    rm -f /tmp/protoc.zip && \
    chmod +x /third_party/protoc/bin/protoc

# Set PATH to include protoc
ENV PATH="/third_party/protoc/bin:${PATH}"

# Install protoc-gen-go and protoc-gen-grpc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION} && \
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GRPC_VERSION}

WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
  go build -ldflags="-s -w" -o bin/sentinel cmd/server/main.go

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/sentinel ./sentinel

RUN mkdir -p /app/certs /app/logs

EXPOSE 8080 50051

ENTRYPOINT ["/app/sentinel"]
