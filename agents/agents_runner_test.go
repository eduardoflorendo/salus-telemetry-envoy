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

//go:generate mockgen -source=agents.go -destination=agents_mock_test.go -package agents_test
package agents_test

import (
	"github.com/golang/mock/gomock"
	"github.com/racker/telemetry-envoy/agents"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestAgentsRunner_ProcessInstall(t *testing.T) {
	var tests = []struct {
		name      string
		agentType telemetry_edge.AgentType
		url       string
		version   string
		exe       string
	}{
		{
			name:      "telegraf_linux",
			agentType: telemetry_edge.AgentType_TELEGRAF,
			url:       "https://dl.influxdata.com/telegraf/releases/telegraf-1.8.0_linux_amd64.tar.gz",
			version:   "1.8.0",
			exe:       "./telegraf/usr/bin/telegraf",
		},
		{
			name:      "telegraf_macos",
			agentType: telemetry_edge.AgentType_TELEGRAF,
			url:       "https://homebrew.bintray.com/bottles/telegraf-1.8.0.high_sierra.bottle.tar.gz",
			version:   "1.8.0",
			exe:       "telegraf/1.8.0/bin/telegraf",
		},
		{
			name:      "filebeat_linux",
			agentType: telemetry_edge.AgentType_FILEBEAT,
			url:       "https://artifacts.elastic.co/downloads/beats/filebeat/filebeat-6.4.1-linux-x86_64.tar.gz",
			version:   "6.4.1",
			exe:       "filebeat-6.4.1-linux-x86_64/filebeat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			agents.UnregisterAllAgentRunners()

			dataPath, err := ioutil.TempDir("", "test_agents")
			require.NoError(t, err)
			defer os.RemoveAll(dataPath)
			viper.Set("agents.dataPath", dataPath)

			agentsRunner, err := agents.NewAgentsRunner()
			require.NoError(t, err)
			require.NotNil(t, agentsRunner)

			mockSpecificAgentRunner := NewMockSpecificAgentRunner(mockCtrl)
			agents.RegisterAgentRunnerForTesting(tt.agentType, mockSpecificAgentRunner)

			install := &telemetry_edge.EnvoyInstructionInstall{
				Url: tt.url,
				Exe: tt.exe,
				Agent: &telemetry_edge.Agent{
					Version: tt.version,
					Type:    tt.agentType,
				},
			}

			mockSpecificAgentRunner.EXPECT().EnsureRunning(gomock.Any())

			agentsRunner.ProcessInstall(install)

			_, exeFilename := path.Split(tt.exe)
			assert.FileExists(t, path.Join(dataPath, "agents", tt.agentType.String(), tt.version, "bin", exeFilename))
		})
	}
}
