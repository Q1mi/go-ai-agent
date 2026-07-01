.PHONY: run review build test

QUESTION ?= 请解释 M05 的三 Agent 范式
FILE ?=

run:
	go run ./cmd/reviewagent "$(QUESTION)"

review:
	go run ./cmd/reviewagent --file "$(FILE)"

build:
	go build ./cmd/reviewagent

test:
	go test -race ./...
