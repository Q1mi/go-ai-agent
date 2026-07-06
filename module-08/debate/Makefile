QUESTION ?= 我们准备用单体架构起步、后期再拆微服务，这个决策有哪些风险？
ROUNDS ?= 3
PROVIDER ?= auto
TRANSCRIPT ?= transcript.md

.PHONY: run offline openai transcript build test fmt tidy vet

run:
	go run ./cmd/debate -provider $(PROVIDER) -rounds $(ROUNDS) "$(QUESTION)"

offline:
	go run ./cmd/debate -provider offline -rounds $(ROUNDS) "$(QUESTION)"

openai:
	go run ./cmd/debate -provider openai -rounds $(ROUNDS) "$(QUESTION)"

transcript:
	go run ./cmd/debate -provider $(PROVIDER) -rounds $(ROUNDS) -transcript $(TRANSCRIPT) "$(QUESTION)"

build:
	mkdir -p bin
	go build -o bin/debate ./cmd/debate

test:
	go test -race ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy

vet:
	go vet ./...
