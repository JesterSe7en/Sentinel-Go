BINARY_NAME=sentinel
BIN_DIR=bin
CMD_DIR=cmd/server

.PHONY: proto
proto:
	protoc --go_out=. --go-grpc_out=. api/limiter.proto

.PHONY: build
build: proto
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go

.PHONY: run
run: build
	./$(BIN_DIR)/$(BINARY_NAME)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
