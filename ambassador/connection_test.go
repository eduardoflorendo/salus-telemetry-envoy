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

package ambassador_test

import (
	"context"
	"github.com/petergtz/pegomock"
	"github.com/phayes/freeport"
	"github.com/racker/telemetry-envoy/ambassador"
	"github.com/racker/telemetry-envoy/config"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	netContext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"net"
	"strconv"
	"testing"
	"time"
)

type TestingAmbassadorService struct {
	done       chan struct{}
	attaches   chan *telemetry_edge.EnvoySummary
	keepAlives chan *telemetry_edge.KeepAliveRequest
	logs       chan *telemetry_edge.LogEvent
	metrics    chan *telemetry_edge.PostedMetric
}

func NewTestingAmbassadorService(done chan struct{}) *TestingAmbassadorService {
	return &TestingAmbassadorService{
		done:       done,
		attaches:   make(chan *telemetry_edge.EnvoySummary, 1),
		keepAlives: make(chan *telemetry_edge.KeepAliveRequest, 1),
		logs:       make(chan *telemetry_edge.LogEvent, 1),
		metrics:    make(chan *telemetry_edge.PostedMetric, 1),
	}
}

func (s *TestingAmbassadorService) AttachEnvoy(summary *telemetry_edge.EnvoySummary, resp telemetry_edge.TelemetryAmbassador_AttachEnvoyServer) error {
	s.attaches <- summary
	<-s.done
	return nil
}

func (s *TestingAmbassadorService) KeepAlive(ctx netContext.Context, req *telemetry_edge.KeepAliveRequest) (*telemetry_edge.KeepAliveResponse, error) {
	s.keepAlives <- req
	return &telemetry_edge.KeepAliveResponse{}, nil
}

func (s *TestingAmbassadorService) PostLogEvent(ctx netContext.Context, log *telemetry_edge.LogEvent) (*telemetry_edge.PostLogEventResponse, error) {
	s.logs <- log
	return &telemetry_edge.PostLogEventResponse{}, nil
}

func (s *TestingAmbassadorService) PostMetric(ctx netContext.Context, metric *telemetry_edge.PostedMetric) (*telemetry_edge.PostMetricResponse, error) {
	s.metrics <- metric
	return &telemetry_edge.PostMetricResponse{}, nil
}

func TestStandardEgressConnection_Start(t *testing.T) {
	pegomock.RegisterMockTestingT(t)

	ambassadorPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	ambassadorAddr := net.JoinHostPort("localhost", strconv.Itoa(ambassadorPort))
	listener, err := net.Listen("tcp", ambassadorAddr)
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	var done = make(chan struct{}, 1)
	defer close(done)
	ambassadorService := NewTestingAmbassadorService(done)
	telemetry_edge.RegisterTelemetryAmbassadorServer(grpcServer, ambassadorService)

	go grpcServer.Serve(listener)
	defer grpcServer.Stop()

	idGenerator := NewMockIdGenerator()
	pegomock.When(idGenerator.Generate()).ThenReturn("id-1")

	mockAgentsRunner := NewMockRouter()
	viper.Set(config.AmbassadorAddress, ambassadorAddr)
	viper.Set("tls.disabled", true)
	viper.Set("ambassador.keepAliveInterval", 1*time.Millisecond)
	egressConnection, err := ambassador.NewEgressConnection(mockAgentsRunner, idGenerator)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go egressConnection.Start(ctx, []telemetry_edge.AgentType{telemetry_edge.AgentType_TELEGRAF})
	defer cancel()

	select {
	case summary := <-ambassadorService.attaches:
		assert.Equal(t, "id-1", summary.InstanceId)
	case <-time.After(500 * time.Millisecond):
		t.Error("did not see attachment in time")
	}

	select {
	case <-ambassadorService.keepAlives:
		// good
	case <-time.After(100 * time.Millisecond):
		t.Error("did not see keep alive in time")
	}
}

func TestStandardEgressConnection_PostMetric(t *testing.T) {
	pegomock.RegisterMockTestingT(t)

	ambassadorPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	ambassadorAddr := net.JoinHostPort("localhost", strconv.Itoa(ambassadorPort))
	listener, err := net.Listen("tcp", ambassadorAddr)
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	done := make(chan struct{}, 1)
	defer close(done)
	ambassadorServer := NewTestingAmbassadorService(done)
	telemetry_edge.RegisterTelemetryAmbassadorServer(grpcServer, ambassadorServer)

	go grpcServer.Serve(listener)
	defer grpcServer.Stop()

	idGenerator := NewMockIdGenerator()
	pegomock.When(idGenerator.Generate()).ThenReturn("id-1")

	mockAgentsRunner := NewMockRouter()
	viper.Set(config.AmbassadorAddress, ambassadorAddr)
	viper.Set("tls.disabled", true)
	egressConnection, err := ambassador.NewEgressConnection(mockAgentsRunner, idGenerator)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go egressConnection.Start(ctx, []telemetry_edge.AgentType{telemetry_edge.AgentType_TELEGRAF})
	defer cancel()

	select {
	case <-ambassadorServer.attaches:
		//continue
	case <-time.After(500 * time.Millisecond):
		t.Log("did not see attachment in time")
		t.FailNow()
	}

	metric := &telemetry_edge.Metric{
		Variant: &telemetry_edge.Metric_NameTagValue{
			NameTagValue: &telemetry_edge.NameTagValueMetric{
				Name: "cpu",
				Tags: map[string]string{
					"cpu": "cpu1",
				},
				Fvalues: map[string]float64{
					"usage": 12.34,
				},
			},
		},
	}
	egressConnection.PostMetric(metric)

	select {
	case postedMetric := <-ambassadorServer.metrics:
		assert.Equal(t, "id-1", postedMetric.InstanceId)
		assert.Equal(t, "cpu", postedMetric.Metric.GetNameTagValue().Name)

	case <-time.After(100 * time.Millisecond):
		t.Error("did not see posted metric in time")
	}
}
