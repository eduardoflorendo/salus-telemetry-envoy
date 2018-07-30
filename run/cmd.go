package run

import (
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
	"log"
)

type RunConfig struct {
}

func RegisterCommand(app *kingpin.Application) *kingpin.CmdClause {
	cfg := NewEnvoyRunnerConfig()
	cmd := app.Command("run", "Runs the telemetry-envoy service").
		Action(func(ctxt *kingpin.ParseContext) error {
			runner, err := NewEnvoyRunner(cfg)
			if err != nil {
				log.Fatal("failed to instantiate runner", err)
			}

			err = runner.Run()
			if err != nil {
				runner.log.Warn("terminating", zap.Error(err))
			}
			return nil
		})

	cmd.Flag("ambassador", "The host:port of the Telemetry Ambassador").
		Required().StringVar(&cfg.AmbassadorAddress)

	cmd.Flag("ca", "Ambassador CA cert").
		Required().ExistingFileVar(&cfg.CaPath)
	cmd.Flag("cert", "Envoy's cert").
		Required().ExistingFileVar(&cfg.CertPath)
	cmd.Flag("key", "Envoy's private key").
		Required().ExistingFileVar(&cfg.KeyPath)

	cmd.Flag("lumberjack-bind", "The host:port to bind for lumberjack serving").
		StringVar(&cfg.LumberjackBind)

	return cmd
}
