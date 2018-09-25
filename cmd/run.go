package cmd

import (
	"context"
	"fmt"
	"github.com/racker/telemetry-envoy/agents"
	"github.com/racker/telemetry-envoy/ambassador"
	"github.com/racker/telemetry-envoy/ingest"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/signal"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the Envoy with a secure connection",
	Run: func(cmd *cobra.Command, args []string) {

		handleInterrupts(func(ctx context.Context) {

			agentsRunner := agents.NewAgentsRunner()
			connection, err := ambassador.NewConnection(agentsRunner)
			if err != nil {
				log.WithError(err).Fatal("Unable to setup ambassador connection")
			}

			lumberjack, err := ingest.NewLumberjack(connection)
			if err != nil {
				log.WithError(err).Fatal("Unable to setup lumberjack ingest")
			}

			go lumberjack.Start(ctx)
			go connection.Start(ctx)

		})
	},
}

func handleInterrupts(body func(ctx context.Context)) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	rootCtx, cancel := context.WithCancel(context.Background())

	body(rootCtx)

	for {
		select {
		case <-signalChan:
			fmt.Println("Cancelling application context")
			cancel()
		case <-rootCtx.Done():
			os.Exit(0)
		}
	}

}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().String("ca", "", "Ambassador CA certificate")
	viper.BindPFlag("tls.ca", runCmd.Flag("ca"))

	runCmd.Flags().String("cert", "", "Certificate to use for authentication")
	viper.BindPFlag("tls.cert", runCmd.Flag("cert"))

	runCmd.Flags().String("key", "", "Private key to use for authentication")
	viper.BindPFlag("tls.key", runCmd.Flag("key"))
}
