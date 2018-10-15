OS := $(shell uname -s)

.PHONY: default
default: generate build

.PHONY: snapshot
snapshot:
	goreleaser release --rm-dist --snapshot

.PHONY: build
build:
	go build -o telemetry-envoy .

.PHONY: generate
generate:
	go generate ./...

.PHONY: clean
clean:
	rm -f telemetry-envoy
	rm -rf */matchers
	rm -f */mock_*_test.go

.PHONY: test
test: clean generate
	go test ./...

.PHONY: test-verbose
test-verbose: generate
	go test -v ./...

.PHONY: coverage
coverage: generate
	go test -cover ./...

.PHONY: coverage-report
coverage-report: generate
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: init-os-specific init-gotools init
init: init-os-specific init-gotools

ifeq (${OS},Darwin)
init-os-specific:
	-brew install protobuf goreleaser
else
ifeq (${OS},Linux)
init-os-specific:
	sudo apt install protobuf-compiler
endif
endif

init-gotools:
	go install github.com/golang/protobuf/protoc-gen-go
	go install github.com/petergtz/pegomock/pegomock