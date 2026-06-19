.PHONY: build build-linux clean run-tui run-web

BINARY=llm-api-uptime
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	go build -ldflags="-s -w" -o $(BINARY).exe .

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY) .

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf data/

run-tui: build
	./$(BINARY).exe -mode=tui

run-web: build
	WEB_ENABLED=true ./$(BINARY).exe -mode=web

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run
