package run

import (
	"context"
	"encoding/json"
	"github.com/elastic/go-lumber/lj"
	"github.com/elastic/go-lumber/server"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"go.uber.org/zap"
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
			r.processLumberjackBatch(batch)
		}
	}
}

func (r *EnvoyRunner) processLumberjackBatch(batch *lj.Batch) {
	r.log.Debug("received lumberjack batch", zap.Int("batchSize", len(batch.Events)))
	for _, event := range batch.Events {
		eventBytes, err := json.Marshal(event)
		if err != nil {
			r.log.Warn("couldn't marshal", zap.Error(err))
		} else {
			r.log.Debug("lumberjack event", zap.String("event", string(eventBytes)))
		}

		callCtx, callCancel := context.WithTimeout(r.rootCtx, r.config.GrpcCallLimit)

		r.log.Debug("posting log event")
		_, err = r.client.PostLogEvent(callCtx, &telemetry_edge.LogEvent{
			InstanceId:  r.instanceId,
			AgentType:   telemetry_edge.AgentType_FILEBEAT,
			JsonContent: string(eventBytes),
		})
		if err != nil {
			r.log.Warn("failed to post log event", zap.Error(err))
		}
		callCancel()
	}
	batch.ACK()
}
