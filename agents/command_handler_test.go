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

package agents_test

import (
	"bytes"
	"context"
	"github.com/petergtz/pegomock"
	"github.com/racker/telemetry-envoy/agents"
	"github.com/racker/telemetry-envoy/agents/matchers"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"
)

func TestStandardCommandHandler_StartAgentCommand(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)

	dataPath, err := ioutil.TempDir("", "test_agents")
	require.NoError(t, err)
	defer os.RemoveAll(dataPath)

	cmdCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	markerPath := path.Join(dataPath, "marker")
	cmd := exec.CommandContext(cmdCtx, "./telegraf", markerPath)
	cmd.Dir = "testdata"

	commandHandler := agents.NewCommandHandler()
	err = commandHandler.StartAgentCommand(cmdCtx, cmd, telemetry_edge.AgentType_TELEGRAF, "Agent Config:", 1*time.Second)
	require.NoError(t, err)

	assert.FileExists(t, markerPath)

	assert.NotZero(t, logBuffer.Len())
	assert.Contains(t, logBuffer.String(), "Agent Config:")
}

func TestStandardCommandHandler_StartAgentCommand_NoWaitFor(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)

	dataPath, err := ioutil.TempDir("", "test_agents")
	require.NoError(t, err)
	defer os.RemoveAll(dataPath)

	cmdCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	markerPath := path.Join(dataPath, "marker")
	cmd := exec.CommandContext(cmdCtx, "./filebeat", markerPath)
	cmd.Dir = "testdata"

	commandHandler := agents.NewCommandHandler()
	err = commandHandler.StartAgentCommand(cmdCtx, cmd, telemetry_edge.AgentType_FILEBEAT, "", 0)
	require.NoError(t, err)

	sawMarker := make(chan struct{})
	go func() {
		for {
			if strings.Contains(logBuffer.String(), "Created marker file") {
				close(sawMarker)
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	select {
	case <-time.After(1 * time.Second):
		t.Fail()
	case <-sawMarker:
		// good
	}

	assert.FileExists(t, markerPath)
}

func TestStandardCommandHandler_WaitOnAgentCommand(t *testing.T) {
	pegomock.RegisterMockTestingT(t)

	agents.SetAgentRestartDelay(1 * time.Millisecond)

	commandHandler := agents.NewCommandHandler()

	ctx := context.Background()
	agentRunner := NewMockSpecificAgentRunner()

	cmdCtx, _ := context.WithCancel(ctx)

	cmd := exec.CommandContext(cmdCtx, "./sleep_a_little")
	cmd.Dir = "testdata"

	cmd.Run()

	commandHandler.WaitOnAgentCommand(ctx, agentRunner, cmd)

	// allow for agent restart delay and call to EnsureRunning
	time.Sleep(10 * time.Millisecond)

	agentRunner.VerifyWasCalledOnce().Stop()
	agentRunner.VerifyWasCalledOnce().EnsureRunning(matchers.AnyContextContext())
}
