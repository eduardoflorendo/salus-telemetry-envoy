
[![CircleCI branch](https://img.shields.io/circleci/project/github/racker/salus-telemetry-envoy/master.svg)](https://circleci.com/gh/racker/salus-telemetry-envoy)

## Run Configuration

Envoy's `run` sub-command can accept some configuration via command-line arguments and/or all
configuration via a yaml file passed via `--config`. The following is an example configuration
file that can be used as a starting point:

```yaml
resource_id: "xen-id:1234-5678-9012" # The identifier of the resource where this Envoy is running
                                     # The convention is a name:value, but is not required.
labels:
  # Any key:value labels can be provided here an can override discovered labels, such as hostname
  #environment: production
tls:
  auth_service:
    url: http://localhost:8182
    token_provider: keystone_v2
  #Provides client authentication certificates pre-allocated. Remove auth_service config when using this.
  #provided:
    #cert: client.pem
    #key: client-key.pem
    #ca: ca.pem
  token_providers:
    keystone_v2:
      identityServiceUrl: https://identity.api.rackspacecloud.com/v2.0/
      # can also be set by env variable $ENVOY_KEYSTONE_USERNAME
      username: ...
      # can also be set by env variable $ENVOY_KEYSTONE_APIKEY
      apikey: ...
    #Specifies one or more HTTP request headers to pass to authentication service
    #static:
    #  - name: Header-Name
    #    value: headerValue
ambassador:
  address: localhost:6565
ingest:
  lumberjack:
    bind: localhost:5044
  telegraf:
    json:
      bind: localhost:8094
agent:
  terminationTimeout: 5s
  restartDelay: 1s
```

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

This module is actually a submodule of the [salus-telemetry-bundle] (https://github.com/racker/salus-telemetry-bundle).  The following instructions expect that you have installed that repo with this as a submodule.

When using IntelliJ, install the Go plugin from JetBrains and create a run configuration
by right-clicking on the `main.go` file and choosing the "Create ..." option under the
run options.

Choose "Directory" for the "Run Kind"

For ease of configuration, you'll want to set the working directory of the run configuration
to be the `dev` directory of the `telemetry-bundle` project.

Add the following to the "Program arguments":

```
run --config=envoy-config-provided.yml
```

The `envoy-config-provided.yml` can be replaced with one of the other config files located there depending on
the scenario currently running on your system.

### Running from command-line

Build and install the executable by running:

```bash
make install
```

Ensure you have `$GOPATH/bin` in your `$PATH` in order to reference the executable installed by `make install`.

Go over to the bundle repo's `dev` directory and run the built envoy from there:

```bash
cd ../../dev
telemetry-envoy run --debug --config=envoy-config-provided.yml
```

The `envoy-config-provided.yml` can be replaced with one of the other config files located there depending on
the scenario currently running on your system.

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

### Release packages

To perform a new release of the packages you must first push a new tag to github and then run `make release`.  For example:

```
git tag -a 0.1.1 -m 'Testing goreleaser'
git push --tags
make release
``` 

The packages should now be available at https://github.com/racker/salus-telemetry-envoy/releases.

## Architecture
The envoy operates as a single go process with multiple goroutines handling the subfunctions.  It is diagrammed [here](./doc/envoy.png)

### Connection
Responsible for the initial attachment to the ambassador and all communication with it, including receiving config/install instructions for the agents, and passing back log messages/metrics from the ingestors.

### Router
Recieves config/install instructions from the Ambassador, (through the Connection,) and forwards them to the appropriate agentRunner.

### agentRunner
There is one of these for each agent managed by the envoy.  The runners are responsible for managing the agents, which are invoked as child processes of the envoy.  The runners use a library called the command handler that abstracts out all the common interprocess communication functions required by the runners.

### Ingestors
These are tcp servers that receive the log/metric data from the agents and forward them back to the Connection for transmittal back to the ambassador.

