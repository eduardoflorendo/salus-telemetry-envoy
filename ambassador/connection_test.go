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
	"github.com/racker/telemetry-envoy/ambassador/matchers"
	"github.com/racker/telemetry-envoy/config"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestStandardEgressConnection_Start(t *testing.T) {
	pegomock.RegisterMockTestingT(t)

	ambassadorPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	ambassadorAddr := net.JoinHostPort("localhost", strconv.Itoa(ambassadorPort))
	listener, err := net.Listen("tcp", ambassadorAddr)
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	mockAmbassadorServer := NewMockTelemetryAmbassadorServer()
	telemetry_edge.RegisterTelemetryAmbassadorServer(grpcServer, mockAmbassadorServer)

	pegomock.When(mockAmbassadorServer.AttachEnvoy(matchers.AnyPtrToTelemetryEdgeEnvoySummary(),
		matchers.AnyTelemetryEdgeTelemetryAmbassadorAttachEnvoyServer())).
		Then(func(params []pegomock.Param) pegomock.ReturnValues {
			// simulate server waiting for instruction observation
			time.Sleep(200 * time.Millisecond)
			return []pegomock.ReturnValue{nil}
		})

	go grpcServer.Serve(listener)
	defer grpcServer.Stop()

	idGenerator := NewMockIdGenerator()
	pegomock.When(idGenerator.Generate()).ThenReturn("id-1")

	mockAgentsRunner := NewMockAgentsRunner()
	viper.Set(config.AmbassadorAddress, ambassadorAddr)
	viper.Set("tls.disabled", true)
	viper.Set("ambassador.keepAliveInterval", 1*time.Millisecond)
	egressConnection, err := ambassador.NewEgressConnection(mockAgentsRunner, idGenerator)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go egressConnection.Start(ctx, []telemetry_edge.AgentType{telemetry_edge.AgentType_TELEGRAF})
	defer cancel()

	// allow for goroutine execution
	time.Sleep(100 * time.Millisecond)

	summary, _ := mockAmbassadorServer.VerifyWasCalledOnce().AttachEnvoy(matchers.AnyPtrToTelemetryEdgeEnvoySummary(),
		matchers.AnyTelemetryEdgeTelemetryAmbassadorAttachEnvoyServer()).GetCapturedArguments()

	assert.Equal(t, "id-1", summary.InstanceId)

	mockAmbassadorServer.VerifyWasCalled(pegomock.AtLeast(1)).
		KeepAlive(matchers.AnyContextContext(), matchers.AnyPtrToTelemetryEdgeKeepAliveRequest())
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

	mockAmbassadorServer := NewMockTelemetryAmbassadorServer()
	telemetry_edge.RegisterTelemetryAmbassadorServer(grpcServer, mockAmbassadorServer)

	pegomock.When(mockAmbassadorServer.AttachEnvoy(matchers.AnyPtrToTelemetryEdgeEnvoySummary(),
		matchers.AnyTelemetryEdgeTelemetryAmbassadorAttachEnvoyServer())).
		Then(func(params []pegomock.Param) pegomock.ReturnValues {
			// simulate server waiting for instruction observation
			time.Sleep(200 * time.Millisecond)
			return []pegomock.ReturnValue{nil}
		})

	go grpcServer.Serve(listener)
	defer grpcServer.Stop()

	idGenerator := NewMockIdGenerator()
	pegomock.When(idGenerator.Generate()).ThenReturn("id-1")

	mockAgentsRunner := NewMockAgentsRunner()
	viper.Set(config.AmbassadorAddress, ambassadorAddr)
	viper.Set("tls.disabled", true)
	egressConnection, err := ambassador.NewEgressConnection(mockAgentsRunner, idGenerator)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go egressConnection.Start(ctx, []telemetry_edge.AgentType{telemetry_edge.AgentType_TELEGRAF})
	defer cancel()

	// allow for connection
	time.Sleep(10 * time.Millisecond)
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

	// allow for goroutine execution
	time.Sleep(100 * time.Millisecond)

	_, postedMetric := mockAmbassadorServer.VerifyWasCalledOnce().
		PostMetric(matchers.AnyContextContext(), matchers.AnyPtrToTelemetryEdgePostedMetric()).
		GetCapturedArguments()

	assert.Equal(t, "id-1", postedMetric.InstanceId)
	require.Equal(t, metric, postedMetric.Metric)
}
