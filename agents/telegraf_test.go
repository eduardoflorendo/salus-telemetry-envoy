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
	"context"
	"github.com/petergtz/pegomock"
	"github.com/racker/telemetry-envoy/agents"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestTelegrafRunner_ProcessConfig(t *testing.T) {

	dataPath, err := ioutil.TempDir("", "telegraf_test")
	require.NoError(t, err)
	defer os.RemoveAll(dataPath)

	runner := &agents.TelegrafRunner{}
	runner.Load(dataPath)
	runner.IngestHost = "localhost"
	runner.IngestPort = 8094

	configure := &telemetry_edge.EnvoyInstructionConfigure{
		AgentType: telemetry_edge.AgentType_TELEGRAF,
		Operations: []*telemetry_edge.ConfigurationOp{
			{
				Id:      "a-b-c",
				Type:    telemetry_edge.ConfigurationOp_MODIFY,
				Content: "configuration content",
			},
		},
	}
	runner.ProcessConfig(configure)

	var files, mainConfigs, instanceConfigs int
	filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		files++
		if filepath.Base(path) == "telegraf.conf" {
			mainConfigs++
			content, err := ioutil.ReadFile(path)
			require.NoError(t, err)

			assert.Contains(t, string(content), "outputs.socket_writer")
			assert.Contains(t, string(content), "address = \"tcp://localhost:8094\"")
		} else if filepath.Base(path) == "a-b-c.conf" {
			instanceConfigs++
			content, err := ioutil.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "configuration content", string(content))

			assert.Equal(t, "config.d", filepath.Base(filepath.Dir(path)))
		}
		return nil
	})
	assert.NotZero(t, files)
	assert.Equal(t, 1, mainConfigs)
	assert.Equal(t, 1, instanceConfigs)
}

func TestTelegrafRunner_EnsureRunning_NoConfig(t *testing.T) {
	pegomock.RegisterMockTestingT(t)

	dataPath, err := ioutil.TempDir("", "test_agents")
	require.NoError(t, err)
	defer os.RemoveAll(dataPath)

	commandHandler := agents.NewMockCommandHandler()

	telegrafRunner := &agents.TelegrafRunner{}
	telegrafRunner.SetCommandHandler(commandHandler)
	telegrafRunner.Load(dataPath)

	ctx := context.Background()
	telegrafRunner.EnsureRunning(ctx)

	// should not see any interaction with command handler
}
