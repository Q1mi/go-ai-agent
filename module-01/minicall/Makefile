.PHONY: run build test

QUESTION ?= 你好

run:
	go run ./cmd/minicall "$(QUESTION)"

build:
	mkdir -p bin
	go build -o bin/minicall ./cmd/minicall

test:
	go test -race ./...
