QUESTION ?= MiniCall 如何配置模型
DOCS ?= ./docs
TOP ?= 5
MODE ?= hybrid

.PHONY: docker-up docker-down init reset index reindex search ask ask-llm agent compare run build test fmt tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

init:
	go run ./cmd/minikb init

reset:
	go run ./cmd/minikb init -reset

index:
	go run ./cmd/minikb index -docs $(DOCS)

reindex:
	go run ./cmd/minikb index -docs $(DOCS) -reset

search:
	go run ./cmd/minikb search -mode $(MODE) -top $(TOP) "$(QUESTION)"

ask:
	go run ./cmd/minikb ask -top $(TOP) "$(QUESTION)"

ask-llm:
	go run ./cmd/minikb ask -llm -top $(TOP) "$(QUESTION)"

agent:
	go run ./cmd/minikb agent "$(QUESTION)"

compare:
	go run ./cmd/minikb compare -top $(TOP) "$(QUESTION)"

run: search

build:
	mkdir -p bin
	go build -o bin/minikb ./cmd/minikb

test:
	go test -race ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy
