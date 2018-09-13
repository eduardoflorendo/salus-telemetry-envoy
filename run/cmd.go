package run

import (
	"gopkg.in/alecthomas/kingpin.v2"
)

func RegisterCommand(app *kingpin.Application) *kingpin.CmdClause {
	cmd := app.Command("run", "Runs the telemetry-envoy service")

	cmd.Flag("ambassador", "The host:port of the Telemetry Ambassador").
		Default("localhost:6565").StringVar(&envoyRunnerConfig.AmbassadorAddress)

	cmd.Flag("ca", "Ambassador CA cert").
		Required().ExistingFileVar(&envoyRunnerConfig.CaPath)
	cmd.Flag("cert", "Envoy's cert").
		Required().ExistingFileVar(&envoyRunnerConfig.CertPath)
	cmd.Flag("key", "Envoy's private key").
		Required().ExistingFileVar(&envoyRunnerConfig.KeyPath)

	cmd.Flag("lumberjack-bind", "The host:port to bind for lumberjack serving").
		Default("localhost:5044").StringVar(&envoyRunnerConfig.LumberjackBind)

	return cmd
}
