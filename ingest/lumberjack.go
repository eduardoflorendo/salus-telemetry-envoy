package ingest

import (
	"context"
	"encoding/json"
	"github.com/elastic/go-lumber/lj"
	"github.com/elastic/go-lumber/server"
	"github.com/racker/telemetry-envoy/ambassador"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Lumberjack struct {
	connection *ambassador.Connection
	server     server.Server
}

func init() {
	viper.SetDefault("lumberjack.bind", "localhost:5044")
}

func NewLumberjack(connection *ambassador.Connection) (*Lumberjack, error) {
	lumberjack := &Lumberjack{
		connection: connection,
	}

	address := viper.GetString("lumberjack.bind")

	var err error
	lumberjack.server, err = server.ListenAndServe(address, server.V2(true))
	if err != nil {
		return nil, err
	}

	log.WithField("address", address).Info("Listening for lumberjack")
	return lumberjack, nil
}

// Start processes incoming lumberjack batches
func (l *Lumberjack) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			l.server.Close()
			return

		case batch := <-l.server.ReceiveChan():
			l.processLumberjackBatch(batch)
		}
	}
}

func (l *Lumberjack) processLumberjackBatch(batch *lj.Batch) {
	log.WithField("batchSize", len(batch.Events)).Debug("received lumberjack batch")
	for _, event := range batch.Events {
		eventBytes, err := json.Marshal(event)
		if err != nil {
			log.WithError(err).Warn("couldn't marshal")
		} else {
			log.WithField("event", string(eventBytes)).Debug("lumberjack event")
		}

		l.connection.PostLogEvent(telemetry_edge.AgentType_FILEBEAT,
			string(eventBytes))
	}
	batch.ACK()
}
