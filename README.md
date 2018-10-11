## Development

### Tooling

First, install Go 1.11 (or newer). On MacOS you can install with `brew install golang`.

After that you can [install the gRPC compiler tooling for Go](https://grpc.io/docs/quickstart/go.html#install-protocol-buffers-v3) 
and [goreleaser](https://goreleaser.com/). 
On MacOS you can install both using `make init`.

### IntelliJ Run Config

When using IntelliJ, install the Go plugin from JetBrains and create a run configuration
by right-clicking on the `main.go` file and choosing the "Create ..." option under the
run options.

Choose "Directory" for the "Run Kind"

For ease of configuration, you'll want to set the working directory of the run configuration
to be the `dev-support` directory of the `telemetry-core` project.

Add the following to the "Program arguments":

```
run --config=envoy-config.yml
```

### gRPC code generating

When first setting up the project and after the `telemetry_edge/telemetry-edge.proto` file
is changed, you will need to re-generate the Go source from it using

```
make generate
```

### Executable build

You can build a `telemetry-envoy` executable by using

```
make build
```

### Cross-platform build

If you need to build an executable for all supported platforms, such as for Linux when
developing on MacOS, use

```
make snapshot
```