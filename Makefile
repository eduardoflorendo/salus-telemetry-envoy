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

ifeq (${OS},Darwin)
.PHONY: init
init:
	-brew install protobuf goreleaser
	go install github.com/golang/protobuf/protoc-gen-go
else
ifeq (${OS},Linux)
init:
	sudo apt install protobuf-compiler
	go install github.com/golang/protobuf/protoc-gen-go
endif
endif
