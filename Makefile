BINARY_NAME=sentinel
BIN_DIR=bin
CMD_DIR=cmd/server

PROTOC_VERSION = 35.0
PROTOC_NAME=$(PROTOC_VERSION)-$(PROTOC_PLATFORM)
PROTOC_URL=https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_NAME).zip

PROTOC_GEN_GO_VERSION=v1.36.11
PROTOC_GEN_GRPC_VERSION=v1.6.2


OS   := $(shell uname -s)
ARCH := $(shell uname -m)

ifeq ($(OS),Darwin)
  PROTOC_PLATFORM = osx-aarch_64
else ifeq ($(OS),Linux)
  ifeq ($(ARCH),x86_64)
    PROTOC_PLATFORM = linux-x86_64
  else ifeq ($(ARCH),aarch64)
    PROTOC_PLATFORM = linux-aarch_64
  endif
endif

.PHONY: install-protoc
install-protoc:
	@echo "Installing protoc..."
	@if [ ! -f "third_party/protoc/bin/protoc" ]; then \
		mkdir -p third_party/protoc && \
		curl -L $(PROTOC_URL) -o third_party/protoc/protoc.zip && \
		unzip third_party/protoc/protoc.zip -d third_party/protoc && \
		rm third_party/protoc/protoc.zip; \
	else \
		echo "protoc already installed"; \
	fi

.PHONY: install-go-plugins
install-go-plugins:
	@echo "Installing protoc-gen-go and protoc-gen-grpc..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GRPC_VERSION)

.PHONY: proto
proto: install-protoc install-go-plugins
	@echo "Generating protobuf code..."
	@third_party/protoc/bin/protoc --go_out=. --go-grpc_out=. api/v1/limiter.proto

.PHONY: lint
lint:
	@echo "Linting..."
	@go vet ./...

.PHONY: test
test:
	@go test ./...

.PHONY: test-verbose
test-verbose:
	@go test -v ./...

.PHONY: build
build: proto
	@echo "Building..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@if [ $$? -eq 0 ]; then \
		echo "Binary built successfully!"; \
	fi

.PHONY: run
run: build
	./$(BIN_DIR)/$(BINARY_NAME)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

.PHONY: docker-build
docker-build: build
	@echo "Building Docker image for arm64..."
	@docker buildx build \
		-f Dockerfile \
		--platform linux/arm64 \
		-t sentinel-go:latest \
		--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
		--build-arg PROTOC_GEN_GO_VERSION=$(PROTOC_GEN_GO_VERSION) \
		--build-arg PROTOC_GEN_GRPC_VERSION=$(PROTOC_GEN_GRPC_VERSION) \
		--load .
	@echo "Docker image for arm64 built successfully!"
