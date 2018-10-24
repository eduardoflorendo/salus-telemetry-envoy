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

package ambassador

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/agents"
	"github.com/racker/telemetry-envoy/auth"
	"github.com/racker/telemetry-envoy/config"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"runtime"
	"strings"
	"time"
)

type EgressConnection interface {
	Start(ctx context.Context, supportedAgents []telemetry_edge.AgentType)
	PostLogEvent(agentType telemetry_edge.AgentType, jsonContent string)
	PostMetric(metric *telemetry_edge.Metric)
}

type IdGenerator interface {
	Generate() string
}

type StandardIdGenerator struct{}

func NewIdGenerator() IdGenerator {
	return &StandardIdGenerator{}
}

func (g *StandardIdGenerator) Generate() string {
	return uuid.NewV1().String()
}

type StandardEgressConnection struct {
	Address           string
	TlsDisabled       bool
	GrpcCallLimit     time.Duration
	KeepAliveInterval time.Duration

	client            telemetry_edge.TelemetryAmbassadorClient
	instanceId        string
	ctx               context.Context
	agentsRunner      agents.AgentsRunner
	grpcTlsDialOption grpc.DialOption
	certificate       *tls.Certificate
	supportedAgents   []telemetry_edge.AgentType
	idGenerator       IdGenerator
}

func init() {
	viper.SetDefault(config.AmbassadorAddress, "localhost:6565")
	viper.SetDefault("grpc.callLimit", 30*time.Second)
	viper.SetDefault("grpc.maxBackoffDelay", 10*time.Second)
	viper.SetDefault("ambassador.keepAliveInterval", 10*time.Second)
}

func NewEgressConnection(agentsRunner agents.AgentsRunner, idGenerator IdGenerator) (EgressConnection, error) {
	connection := &StandardEgressConnection{
		Address:           viper.GetString(config.AmbassadorAddress),
		TlsDisabled:       viper.GetBool("tls.disabled"),
		GrpcCallLimit:     viper.GetDuration("grpc.callLimit"),
		KeepAliveInterval: viper.GetDuration("ambassador.keepAliveInterval"),
		agentsRunner:      agentsRunner,
		idGenerator:       idGenerator,
	}

	var err error
	connection.grpcTlsDialOption, err = connection.loadTlsDialOption()
	if err != nil {
		return nil, err
	}

	return connection, nil
}

func (c *StandardEgressConnection) Start(ctx context.Context, supportedAgents []telemetry_edge.AgentType) {
	c.ctx = ctx
	c.supportedAgents = supportedAgents

	for {
		backoff.RetryNotify(c.attach, backoff.WithContext(backoff.NewExponentialBackOff(), c.ctx),
			func(err error, delay time.Duration) {
				log.WithError(err).WithField("delay", delay).Warn("delaying until next attempt")

				cause := errors.Cause(err)
				if cause != nil && cause != err {
					if strings.Contains(cause.Error(), "tls: expired certificate") {
						log.Warn("authenticating certificate has expired, reloading certificates")

						var loadErr error
						c.grpcTlsDialOption, loadErr = c.loadTlsDialOption()
						if loadErr != nil {
							log.WithError(loadErr).Warn("failed to reload certificates")
						}
					}
				}
			})

		c.instanceId = c.idGenerator.Generate()
	}
}

func (c *StandardEgressConnection) attach() error {

	log.WithField("address", c.Address).Info("dialing ambassador")
	// use a blocking dial, but fail on non-temp errors so that we can catch connectivity errors here rather than during
	// the attach operation
	conn, err := grpc.Dial(c.Address,
		c.grpcTlsDialOption,
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
	)
	if err != nil {
		return errors.Wrap(err, "failed to dial Ambassador")
	}
	defer conn.Close()

	c.client = telemetry_edge.NewTelemetryAmbassadorClient(conn)
	c.instanceId = c.idGenerator.Generate()

	connCtx, cancelFunc := context.WithCancel(c.ctx)

	envoySummary := &telemetry_edge.EnvoySummary{
		InstanceId:      c.instanceId,
		SupportedAgents: c.supportedAgents,
		Labels:          c.computeLabels(),
	}
	log.WithField("summary", envoySummary).Info("attaching")

	instructions, err := c.client.AttachEnvoy(connCtx, envoySummary)
	if err != nil {
		return errors.Wrap(err, "failed to attach Envoy")
	}

	errChan := make(chan error, 10)

	go c.watchForInstructions(connCtx, errChan, instructions)
	go c.sendKeepAlives(connCtx, errChan)

	for {
		select {
		case <-connCtx.Done():
			instructions.CloseSend()
			return fmt.Errorf("closed")

		case err := <-errChan:
			log.WithError(err).Warn("terminating")
			cancelFunc()
		}
	}
}

func (c *StandardEgressConnection) PostLogEvent(agentType telemetry_edge.AgentType, jsonContent string) {
	callCtx, callCancel := context.WithTimeout(c.ctx, c.GrpcCallLimit)
	defer callCancel()

	log.Debug("posting log event")
	_, err := c.client.PostLogEvent(callCtx, &telemetry_edge.LogEvent{
		InstanceId:  c.instanceId,
		AgentType:   agentType,
		JsonContent: jsonContent,
	})
	if err != nil {
		log.WithError(err).Warn("failed to post log event")
	}
}

func (c *StandardEgressConnection) PostMetric(metric *telemetry_edge.Metric) {
	callCtx, callCancel := context.WithTimeout(c.ctx, c.GrpcCallLimit)
	defer callCancel()

	log.WithField("metric", metric).Debug("posting metric")
	_, err := c.client.PostMetric(callCtx, &telemetry_edge.PostedMetric{
		InstanceId: c.instanceId,
		Metric:     metric,
	})
	if err != nil {
		log.WithError(err).Warn("failed to post metric")
	}
}

func (c *StandardEgressConnection) sendKeepAlives(ctx context.Context, errChan chan<- error) {
	for {
		select {
		case <-time.After(c.KeepAliveInterval):
			_, err := c.client.KeepAlive(ctx, &telemetry_edge.KeepAliveRequest{
				InstanceId: c.instanceId,
			})
			if err != nil {
				errChan <- errors.Wrap(err, "failed to send keep alive")
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (c *StandardEgressConnection) computeLabels() map[string]string {
	labels := make(map[string]string)

	labels["os"] = runtime.GOOS
	labels["arch"] = runtime.GOARCH

	hostname, err := os.Hostname()
	if err == nil {
		labels["hostname"] = hostname
	} else {
		log.WithError(err).Warn("unable to determine hostname")
	}

	return labels
}

func (c *StandardEgressConnection) loadTlsDialOption() (grpc.DialOption, error) {
	if c.TlsDisabled {
		return grpc.WithInsecure(), nil
	}

	certificate, certPool, err := auth.LoadCertificates()
	if err != nil {
		return nil, err
	}
	c.certificate = certificate

	transportCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{*certificate},
		RootCAs:      certPool,
	})
	return grpc.WithTransportCredentials(transportCreds), nil
}

func (c *StandardEgressConnection) watchForInstructions(ctx context.Context,
	errChan chan<- error, instructions telemetry_edge.TelemetryAmbassador_AttachEnvoyClient) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			instruction, err := instructions.Recv()
			if err != nil {
				errChan <- errors.Wrap(err, "failed to receive instruction")
				return
			}

			switch {
			case instruction.GetInstall() != nil:
				c.agentsRunner.ProcessInstall(instruction.GetInstall())

			case instruction.GetConfigure() != nil:
				c.agentsRunner.ProcessConfigure(instruction.GetConfigure())

			case instruction.GetRefresh() != nil:
				log.Debug("received refresh") //TODO
			}
		}
	}
}
