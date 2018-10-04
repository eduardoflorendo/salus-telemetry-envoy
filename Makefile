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

.PHONY: test
test: generate
	go test ./...

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
	go install github.com/golang/mock/mockgen