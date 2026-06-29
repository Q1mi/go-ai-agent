.PHONY: run dry-run build test vet

QUESTION ?= 如何修改默认超时？

run:
	go run ./cmd/docqa "$(QUESTION)"

dry-run:
	go run ./cmd/docqa -dry-run "$(QUESTION)"

build:
	mkdir -p bin
	go build -o bin/docqa ./cmd/docqa

test:
	go test -race -count=1 ./...

vet:
	go vet ./...
