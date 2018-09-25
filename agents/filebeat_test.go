package agents_test

import (
	"github.com/racker/telemetry-envoy/agents"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestFilebeatRunner_ProcessConfig(t *testing.T) {

	dataPath, err := ioutil.TempDir("", "filebeat_test")
	require.NoError(t, err)
	defer os.RemoveAll(dataPath)

	runner := agents.NewFilebeatRunner()
	runner.DataPath = dataPath
	runner.LumberjackBind = "localhost:5555"

	configure := &telemetry_edge.EnvoyInstructionConfigure{
		AgentType: telemetry_edge.AgentType_FILEBEAT,
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
		if filepath.Base(path) == "filebeat.yml" {
			mainConfigs++
			content, err := ioutil.ReadFile(path)
			require.NoError(t, err)

			assert.Contains(t, string(content), "path: config.d/*.yml")
			assert.Contains(t, string(content), "hosts: [\"localhost:5555\"]")
		} else if filepath.Base(path) == "a-b-c.yml" {
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
