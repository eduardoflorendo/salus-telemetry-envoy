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
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"testing"
)

func TestAgentsRunner_ProcessInstall(t *testing.T) {
	var tests = []struct {
		name      string
		agentType telemetry_edge.AgentType
		file      string
		version   string
		exe       string
	}{
		{
			name:      "telegraf_linux",
			agentType: telemetry_edge.AgentType_TELEGRAF,
			file:      "telegraf_dot_slash.tgz",
			version:   "1.8.0",
			exe:       "./telegraf/usr/bin/telegraf",
		},
		{
			name:      "filebeat_linux",
			agentType: telemetry_edge.AgentType_FILEBEAT,
			file:      "filebeat_relative.tgz",
			version:   "6.4.1",
			exe:       "filebeat-6.4.1-linux-x86_64/filebeat",
		},
	}

	ts := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	defer ts.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pegomock.RegisterMockTestingT(t)

			agents.UnregisterAllAgentRunners()

			dataPath, err := ioutil.TempDir("", "test_agents")
			require.NoError(t, err)
			defer os.RemoveAll(dataPath)
			viper.Set("agents.dataPath", dataPath)

			agentsRunner, err := agents.NewAgentsRunner()
			require.NoError(t, err)
			require.NotNil(t, agentsRunner)

			mockSpecificAgentRunner := agents.NewMockSpecificAgentRunner()
			agents.RegisterAgentRunnerForTesting(tt.agentType, mockSpecificAgentRunner)

			install := &telemetry_edge.EnvoyInstructionInstall{
				Url: ts.URL + "/" + tt.file,
				Exe: tt.exe,
				Agent: &telemetry_edge.Agent{
					Version: tt.version,
					Type:    tt.agentType,
				},
			}

			agentsRunner.ProcessInstall(install)

			mockSpecificAgentRunner.VerifyWasCalledOnce().EnsureRunning(AnyContext())

			_, exeFilename := path.Split(tt.exe)
			assert.FileExists(t, path.Join(dataPath, "agents", tt.agentType.String(), tt.version, "bin", exeFilename))
		})
	}
}

func AnyContext() context.Context {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*context.Context)(nil)).Elem()))
	return context.Background()
}
