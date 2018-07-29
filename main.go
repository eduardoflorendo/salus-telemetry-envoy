package main

import (
	"fmt"
	"github.com/racker/telemetry-envoy/run"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

var app = kingpin.New("telemetry-envoy", "")

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	run.RegisterCommand(app)
	registerVersionCommand()

	app.Flag("debug", "enable debug level logs").
		Action(func(pctx *kingpin.ParseContext) error {
			logrus.SetLevel(logrus.DebugLevel)
			return nil
		}).Bool()

	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func registerVersionCommand() {
	app.Command("version", "Show current version").
		Action(func(ctx *kingpin.ParseContext) error {
			fmt.Printf("%v, commit %v, built at %v", version, commit, date)
			os.Exit(0)
			return nil
		})
}
