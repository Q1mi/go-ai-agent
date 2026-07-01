.PHONY: run inspector inspector-config build test fmt clean

run:
	go run ./cmd/mcpgo-server

inspector:
	npx @modelcontextprotocol/inspector go run ./cmd/mcpgo-server

inspector-config:
	npx @modelcontextprotocol/inspector --config mcp.json --server mcp-go-demo

build:
	go build -o bin/mcpgo-server ./cmd/mcpgo-server

test:
	go test -race ./...

fmt:
	gofmt -w $$(find . -name '*.go')

clean:
	rm -rf bin
