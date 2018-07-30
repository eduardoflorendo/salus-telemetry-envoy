package main

import (
	"fmt"
	"github.com/racker/telemetry-envoy/run"
	"gopkg.in/alecthomas/kingpin.v2"
	"log"
	"os"
)

var app = kingpin.New("telemetry-envoy", "")

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	runCmd := run.RegisterCommand(app)
	versionCmd := registerVersionCommand()

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {

	case runCmd.FullCommand():
		runner, err := run.NewEnvoyRunner()
		if err != nil {
			log.Fatal("failed to instantiate runner", err)
		}

		err = runner.Run()
		if err != nil {
			log.Fatal("terminating", err)
		}

	case versionCmd.FullCommand():
		fmt.Printf("%v, commit %v, built at %v", version, commit, date)
		os.Exit(0)

	}
}

func registerVersionCommand() *kingpin.CmdClause {
	return app.Command("version", "Show current version")
}
