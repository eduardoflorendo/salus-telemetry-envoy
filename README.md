## Development

### Environment Setup

This application uses Go 1.11 modules, so be sure to clone this **outside** of the `$GOPATH`.

Speaking of which, some of the code generator tooling does expect `$GOPATH` to be set and the tools to
be available in `$PATH`. As such, it is recommended to add the following to your shell's
setup file (such as `~/.profile` or `~/.bashrc`):

```
export GOPATH=$HOME/go
export PATH="$PATH:$GOPATH/bin"
```

### Tooling

First, install Go 1.11 (or newer). On MacOS you can install with `brew install golang`.

After that you can [install the gRPC compiler tooling for Go](https://grpc.io/docs/quickstart/go.html#install-protocol-buffers-v3) 
and [goreleaser](https://goreleaser.com/). 
On MacOS you can install both using `make init`.

### IntelliJ Run Config

If you haven't already, clone the [telemetry-core](https://github.com/racker/rmii-telemetry-core)
repo. For the following example, the core repo is cloned next to the envoy repo.

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

### Running from command-line

Build the executable by running:

```bash
make
```

If you haven't already, clone the [telemetry-core](https://github.com/racker/rmii-telemetry-core)
repo. For the following example, the core repo is cloned next to the envoy repo, but you can
clone it to a location of your choosing.

Go over to the core repo's `dev-support` directory and run the built envoy from there:

```bash
cd ../rmii-telemetry-core/dev-support
../../rmii-telemetry-envoy/telemetry-envoy run --config=envoy-config.yml
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