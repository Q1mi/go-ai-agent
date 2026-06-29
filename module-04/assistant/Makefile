.PHONY: run build test vet

GOAL ?= 现在几点？顺便计算 1+2*3

run:
	go run ./cmd/assistant "$(GOAL)"

react:
	go run ./cmd/assistant -mode react "$(GOAL)"

build:
	mkdir -p bin
	go build -o bin/assistant ./cmd/assistant

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

