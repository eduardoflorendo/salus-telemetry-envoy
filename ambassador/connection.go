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
	"crypto/x509"
	"fmt"
	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/agents"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
	"os"
	"runtime"
	"time"
)

type Connection struct {
	Tls struct {
		Ca, Cert, Key string
	}
	Address           string
	GrpcCallLimit     time.Duration
	KeepAliveInterval time.Duration

	client         telemetry_edge.TelemetryAmbassadorClient
	instanceId     string
	ctx            context.Context
	agentsRunner   *agents.AgentsRunner
	grpcDialOption grpc.DialOption
}

func init() {
	viper.SetDefault("ambassador.address", "localhost:6565")
	viper.SetDefault("ambassador.keepAliveInterval", 10*time.Second)
}

func NewConnection(agentsRunner *agents.AgentsRunner) (*Connection, error) {
	connection := &Connection{
		Address:           viper.GetString("ambassador.address"),
		GrpcCallLimit:     viper.GetDuration("grpc.callLimit"),
		KeepAliveInterval: viper.GetDuration("ambassador.keepAliveInterval"),
		agentsRunner:      agentsRunner,
	}
	viper.UnmarshalKey("tls", &connection.Tls)

	var err error
	connection.grpcDialOption, err = connection.loadTlsDialOption()
	if err != nil {
		return nil, err
	}

	return connection, nil
}

func (c *Connection) Start(ctx context.Context) {
	c.ctx = ctx

	for {
		backoff.RetryNotify(c.attach, backoff.WithContext(backoff.NewExponentialBackOff(), c.ctx),
			func(err error, delay time.Duration) {
				log.WithError(err).WithField("delay", delay).Warn("delaying until next attempt")
			})

		c.instanceId = uuid.NewV1().String()
	}
}

func (c *Connection) attach() error {

	log.WithField("address", c.Address).Info("dialing ambassador")
	conn, err := grpc.Dial(c.Address, c.grpcDialOption)
	if err != nil {
		return errors.Wrap(err, "failed to dial Ambassador")
	}
	defer conn.Close()

	c.client = telemetry_edge.NewTelemetryAmbassadorClient(conn)
	c.instanceId = uuid.NewV1().String()

	connCtx, cancelFunc := context.WithCancel(c.ctx)

	envoySummary := &telemetry_edge.EnvoySummary{
		InstanceId:      c.instanceId,
		SupportedAgents: []telemetry_edge.AgentType{telemetry_edge.AgentType_FILEBEAT},
		Labels:          c.computeLabels(),
	}
	log.WithField("summary", envoySummary).Info("attaching")

	instructions, err := c.client.AttachEnvoy(connCtx, envoySummary)
	if err != nil {
		return errors.Wrap(err, "failed to attach Envoy")
	}

	errChan := make(chan error, 10)

	c.agentsRunner.PurgeAgentConfigs()

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

func (c *Connection) PostLogEvent(agentType telemetry_edge.AgentType, jsonContent string) {
	callCtx, callCancel := context.WithTimeout(c.ctx, c.GrpcCallLimit)

	log.Debug("posting log event")
	_, err := c.client.PostLogEvent(callCtx, &telemetry_edge.LogEvent{
		InstanceId:  c.instanceId,
		AgentType:   agentType,
		JsonContent: jsonContent,
	})
	if err != nil {
		log.WithError(err).Warn("failed to post log event")
	}
	callCancel()
}

func (c *Connection) sendKeepAlives(ctx context.Context, errChan chan<- error) {
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

func (c *Connection) computeLabels() map[string]string {
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

func (c *Connection) loadTlsDialOption() (grpc.DialOption, error) {
	// load ours
	certificate, err := tls.LoadX509KeyPair(
		c.Tls.Cert,
		c.Tls.Key,
	)

	// load the CA
	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(c.Tls.Ca)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read ca cert")
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, errors.Wrap(err, "failed to append certs")
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	})
	return grpc.WithTransportCredentials(transportCreds), nil
}

func (c *Connection) watchForInstructions(ctx context.Context,
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
