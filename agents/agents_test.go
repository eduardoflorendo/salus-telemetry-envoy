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
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentsRunner_ProcessInstall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dataPath, err := ioutil.TempDir("", "test_agents")
	require.NoError(t, err)
	defer os.RemoveAll(dataPath)
	agentsRunner := agents.NewAgentsRunner()
	agentsRunner.DataPath = dataPath

	mockSpecificAgentRunner := NewMockSpecificAgentRunner(mockCtrl)
	agents.InjectMockRunnersIntoAgentRunner(agentsRunner, mockSpecificAgentRunner)

	mockSpecificAgentRunner.EXPECT().EnsureRunning(gomock.Any())

	install := &telemetry_edge.EnvoyInstructionInstall{
		Url: "https://artifacts.elastic.co/downloads/beats/filebeat/filebeat-6.4.1-linux-x86_64.tar.gz",
		Agent: &telemetry_edge.Agent{
			Version: "6.4.1",
			Type:    telemetry_edge.AgentType_FILEBEAT,
		},
		Checksum: "4432c9ad3f9952675706dbc0f959a4128a4c5f018a90eba70746f86c784df39a5bbaf7b1f2e5d1054d57dd5393432fd8475757ba878ca2bdd975865acd183399",
	}
	agentsRunner.ProcessInstall(install)

	var fileCount, exeCount int
	filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		fileCount++
		if filepath.Base(path) == "filebeat" && !info.IsDir() {
			exeCount++
		}
		return nil
	})
	require.NotZero(t, fileCount)
	require.NotZero(t, exeCount)
}
