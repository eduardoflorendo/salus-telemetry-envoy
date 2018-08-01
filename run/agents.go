package run

import (
	"context"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

type AgentRunningInstance struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type AgentsRunner struct {
	config  *EnvoyRunnerConfig
	log     *zap.Logger
	running map[string]*AgentRunningInstance
}

func (ar *AgentsRunner) ensureFilebeatRunning() {

	ar.log.Debug("ensuring filebeat is running")

	agentType := telemetry_edge.AgentType_FILEBEAT.String()

	if _, exists := ar.running[agentType]; exists {
		ar.log.Debug("filebeat is already running")
		return
	}

	agentBasePath := filepath.Join(ar.config.DataPath, agentsSubpath, agentType)
	if !ar.hasRequiredFilebeatPaths(agentBasePath) {
		ar.log.Debug("filebeat not ready to launch due to some missing paths and files")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, filepath.Join(currentVerLink, "filebeat"),
		"run",
		"--path.config", "./",
		"--path.data", "data",
		"--path.logs", "logs")
	cmd.Dir = agentBasePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		ar.log.Warn("failed to start filebeat", zap.Error(err))
		return
	}

	runner := &AgentRunningInstance{
		ctx:    ctx,
		cancel: cancel,
		cmd:    cmd,
	}
	ar.running[agentType] = runner
	ar.log.Debug("started filebeat")
}

func (ar *AgentsRunner) hasRequiredFilebeatPaths(agentBasePath string) bool {
	curVerPath := filepath.Join(agentBasePath, currentVerLink)
	if _, err := os.Stat(curVerPath); os.IsNotExist(err) {
		ar.log.Debug("missing current version link", zap.String("path", curVerPath))
		return false
	}

	configsPath := filepath.Join(agentBasePath, configsSubpath)
	if _, err := os.Stat(configsPath); os.IsNotExist(err) {
		ar.log.Debug("missing configs path", zap.String("path", configsPath))
		return false
	}

	configsDir, err := os.Open(configsPath)
	if err != nil {
		ar.log.Warn("unable to open configs directory for listing", zap.Error(err))
		return false
	}
	defer configsDir.Close()

	names, err := configsDir.Readdirnames(0)
	if err != nil {
		ar.log.Warn("unable to read files in configs directory", zap.Error(err), zap.String("path", configsPath))
		return false
	}

	for _, name := range names {
		if path.Ext(name) == ".yml" {
			return true
		}
	}
	ar.log.Debug("missing config files", zap.String("path", configsPath))
	return false
}

func NewAgentRunner(config *EnvoyRunnerConfig, log *zap.Logger) *AgentsRunner {
	return &AgentsRunner{
		config:  config,
		log:     log,
		running: make(map[string]*AgentRunningInstance),
	}
}
