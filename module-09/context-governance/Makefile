ROUNDS ?= 8
MEMORY ?= .ctx-memory
CSV ?= token-curve.csv

.PHONY: run csv read-memory build test fmt tidy vet clean

run:
	go run ./cmd/ctxdemo -rounds $(ROUNDS) -memory $(MEMORY)

csv:
	go run ./cmd/ctxdemo -rounds $(ROUNDS) -memory $(MEMORY) -csv $(CSV)

read-memory:
	go run ./cmd/ctxdemo -memory $(MEMORY) -read-memory $(ID)

build:
	mkdir -p bin
	go build -o bin/ctxdemo ./cmd/ctxdemo

test:
	go test -race ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy

vet:
	go vet ./...

clean:
	rm -rf bin $(MEMORY) $(CSV)
