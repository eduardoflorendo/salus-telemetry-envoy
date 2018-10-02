/*
 *    Copyright 2018 Rackspace US, Inc.
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 *
 *
 */

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

	registerIngestor(&Lumberjack{})
}

func (l *Lumberjack) Connect(connection *ambassador.Connection) error {
	l.connection = connection

	address := viper.GetString("lumberjack.bind")

	var err error
	l.server, err = server.ListenAndServe(address, server.V2(true))
	if err != nil {
		return err
	}

	log.WithField("address", address).Info("Listening for lumberjack")
	return nil
}

// Start processes incoming lumberjack batches
func (l *Lumberjack) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Info("closing lumberjack ingest")
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
