package main

import (
	"github.com/racker/telemetry-envoy/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Execute(cmd.VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
}
