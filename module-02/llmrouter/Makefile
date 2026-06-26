.PHONY: run build test vet

QUESTION ?= 你好

run:
	go run ./cmd/llmrouter "$(QUESTION)"

build:
	mkdir -p bin
	go build -o bin/llmrouter ./cmd/llmrouter

test:
	go test -race -count=1 ./...

vet:
	go vet ./...
