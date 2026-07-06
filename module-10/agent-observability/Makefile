QUESTION ?= 北京今天需要带伞吗？
JSON ?= spans.json
OTLP_ENDPOINT ?= localhost:4318

ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: jaeger-up jaeger-down jaeger-logs demo demo-jaeger eval eval-jaeger eval-trace eval-trace-jaeger judge attack attack-jaeger json build test fmt tidy vet clean

jaeger-up:
	docker compose up -d

jaeger-down:
	docker compose down

jaeger-logs:
	docker compose logs -f jaeger

demo:
	go run ./cmd/traceagent demo -q "$(QUESTION)"

demo-jaeger:
	go run ./cmd/traceagent demo -exporter jaeger -otlp-endpoint $(OTLP_ENDPOINT) -q "$(QUESTION)"

eval:
	go run ./cmd/traceagent eval

eval-jaeger:
	go run ./cmd/traceagent eval -exporter jaeger -otlp-endpoint $(OTLP_ENDPOINT)

eval-trace:
	go run ./cmd/traceagent eval -show-trace

eval-trace-jaeger:
	go run ./cmd/traceagent eval -exporter jaeger -otlp-endpoint $(OTLP_ENDPOINT) -show-trace

judge:
	go run ./cmd/traceagent eval -judge

attack:
	go run ./cmd/traceagent attack

attack-jaeger:
	go run ./cmd/traceagent attack -exporter jaeger -otlp-endpoint $(OTLP_ENDPOINT)

json:
	go run ./cmd/traceagent demo -q "$(QUESTION)" -json $(JSON)

build:
	mkdir -p bin
	go build -o bin/traceagent ./cmd/traceagent

test:
	go test -race ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy

vet:
	go vet ./...

clean:
	rm -rf bin $(JSON)
