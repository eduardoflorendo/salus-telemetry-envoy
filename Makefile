OS := $(uname -s)

.PHONY: default
default:
	goreleaser release --rm-dist --snapshot

.PHONY: generate
generate:
	go generate ./...

ifeq (${OS}, Darwin)
.PHONY: install-protoc
install-protoc:
	brew install protobuf
	go get -u github.com/golang/protobuf/protoc-gen-go
endif
