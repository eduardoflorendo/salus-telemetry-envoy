OS := $(uname -s)

default: telemetry_edge/telemetry-edge.pb.go

telemetry_edge/telemetry-edge.pb.go: telemetry_edge/telemetry-edge.proto
	protoc -I telemetry_edge/ $< --go_out=plugins=grpc:telemetry_edge

ifeq (${OS}, Darwin)
.PHONY: install-protoc
install-protoc:
	brew install protobuf
	go get -u github.com/golang/protobuf/protoc-gen-go
endif
