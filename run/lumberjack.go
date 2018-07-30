package run

import (
	"context"
	"github.com/elastic/go-lumber/server"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"encoding/json"
)

func (r *EnvoyRunner) handleLumberjack(ctx context.Context, errChan chan<- error) {
	lumberjackServer, err := server.ListenAndServe(r.config.LumberjackBind, server.V2(true))
	if err != nil {
		errChan <- errors.Wrap(err, "unable to create lumberjack server")
		return
	}
	defer lumberjackServer.Close()

	r.log.Info("started lumberjack server", zap.String("address", r.config.LumberjackBind))

	for {
		select {
		case <-ctx.Done():
			return

		case batch := <-lumberjackServer.ReceiveChan():
			r.log.Debug("received lumberjack batch", zap.Int("batchSize", len(batch.Events)))
			for _, event := range batch.Events {
				if m, ok := event.(map[string]interface{}); ok {
					r.log.Debug("it's a map", zap.Any("eventMap", m))
				}
				eventBytes, err := json.Marshal(event)
				if err != nil {
					r.log.Warn("couldn't marshal", zap.Error(err))
				} else {
					r.log.Debug("lumberjack event", zap.String("event", string(eventBytes)))
				}
			}
			//TODO
			batch.ACK()
		}
	}
}
