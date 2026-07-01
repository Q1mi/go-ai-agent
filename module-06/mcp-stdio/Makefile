.PHONY: run-server client-list client-call agent build test fmt clean

QUESTION ?= 现在几点了？顺便计算 12*(3+4)

run-server:
	go run ./cmd/mcpserver

client-list:
	go run ./cmd/mcpclient -list

client-call:
	go run ./cmd/mcpclient -tool calc -args '{"expr":"12*(3+4)"}'

agent:
	go run ./cmd/mcpagent "$(QUESTION)"

build:
	go build -o bin/mcpserver ./cmd/mcpserver
	go build -o bin/mcpclient ./cmd/mcpclient
	go build -o bin/mcpagent ./cmd/mcpagent

test:
	go test -race ./...

fmt:
	gofmt -w $$(find . -name '*.go')

clean:
	rm -rf bin
