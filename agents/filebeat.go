package agents

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"
)

const (
	filebeatMainConfigFilename = "filebeat.yml"
)

type filebeatMainConfigData struct {
	ConfigsPath    string
	LumberjackPort string
}

var filebeatMainConfigTmpl = template.Must(template.New("filebeatMain").Parse(`
filebeat.config.inputs:
  enabled: true
  path: {{.ConfigsPath}}/*.yml
  reload.enabled: true
  reload.period: 5s
output.logstash:
  hosts: ["localhost:{{.LumberjackPort}}"]
`))

type FilebeatRunner struct {
	DataPath       string
	LumberjackBind string
	running        *AgentRunningInstance
}

func NewFilebeatRunner() *FilebeatRunner {
	return &FilebeatRunner{
		DataPath:       viper.GetString("agents.dataPath"),
		LumberjackBind: viper.GetString("lumberjack.bind"),
	}
}

func (fbr *FilebeatRunner) EnsureRunning(ctx context.Context) {
	log.Debug("ensuring filebeat is running")

	agentType := telemetry_edge.AgentType_FILEBEAT.String()

	if fbr.running != nil {
		log.Debug("filebeat is already running")
		return
	}

	agentBasePath := filepath.Join(fbr.DataPath, agentsSubpath, agentType)
	if !fbr.hasRequiredFilebeatPaths(agentBasePath) {
		log.Debug("filebeat not ready to launch due to some missing paths and files")
		return
	}

	cmdCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(cmdCtx, filepath.Join(currentVerLink, "filebeat"),
		"run",
		"--path.config", "./",
		"--path.data", "data",
		"--path.logs", "logs")
	cmd.Dir = agentBasePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.WithField("cmd", cmd).Debug("starting filebeat")
	err := cmd.Start()
	if err != nil {
		log.WithError(err).Warn("failed to start filebeat")
		return
	}

	runner := &AgentRunningInstance{
		ctx:    cmdCtx,
		cancel: cancel,
		cmd:    cmd,
	}
	fbr.running = runner
	log.Info("started filebeat")
}

func (fbr *FilebeatRunner) Stop() {
	if fbr.running != nil {
		fbr.running.cancel()
		fbr.running = nil
	}
}

func (fbr *FilebeatRunner) ProcessConfig(ctx context.Context, configure *telemetry_edge.EnvoyInstructionConfigure) {
	agentType := telemetry_edge.AgentType_FILEBEAT.String()

	agentBasePath := path.Join(fbr.DataPath, agentsSubpath, agentType)
	err := os.MkdirAll(agentBasePath, 0755)
	if err != nil {
		log.WithError(err).WithField("path", agentBasePath).Error("failed to create agent base path")
		return
	}

	configsPath := path.Join(agentBasePath, configsSubpath)
	err = os.MkdirAll(configsPath, 0755)
	if err != nil {
		log.WithError(err).WithField("path", configsPath).Error("failed to create configs path for filebeat")
		return
	}

	mainConfigPath := path.Join(agentBasePath, filebeatMainConfigFilename)
	if _, err := os.Stat(mainConfigPath); os.IsNotExist(err) {
		err = fbr.createMainFilebeatConfig(agentBasePath, mainConfigPath)
		if err != nil {
			log.WithError(err).Error("failed to create main filebeat config")
			return
		}
	}

	for _, op := range configure.GetOperations() {
		log.WithField("op", op).Debug("processing filebeat config operation")

		err = fbr.processFilebeatConfigOp(configsPath, op)
		if err != nil {
			log.WithField("op", op).Warn("failed to process filebeat config operation")
		}
	}

	fbr.EnsureRunning(ctx)

}

func (fbr *FilebeatRunner) processFilebeatConfigOp(configsPath string, op *telemetry_edge.ConfigurationOp) error {
	configInstancePath := filepath.Join(configsPath, fmt.Sprintf("%s.yml", op.GetId()))

	switch op.GetType() {
	case telemetry_edge.ConfigurationOp_MODIFY:
		err := ioutil.WriteFile(configInstancePath, []byte(op.GetContent()), 0644)
		if err != nil {
			return errors.Wrap(err, "failed to write filebeat config file instance")
		}
	}

	return nil
}

func (fbr *FilebeatRunner) createMainFilebeatConfig(agentBasePath, mainConfigPath string) error {

	log.WithField("path", mainConfigPath).Debug("creating main filebeat config file")

	_, port, err := net.SplitHostPort(fbr.LumberjackBind)
	if err != nil {
		return errors.Wrapf(err, "unable to split lumberjack bind info: %v", fbr.LumberjackBind)
	}

	file, err := os.OpenFile(mainConfigPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return errors.Wrap(err, "unable to open main filebeat config file")
	}
	defer file.Close()

	data := filebeatMainConfigData{
		ConfigsPath:    configsSubpath,
		LumberjackPort: port,
	}

	err = filebeatMainConfigTmpl.Execute(file, data)
	if err != nil {
		return errors.Wrap(err, "failed to execute filebeat main config template")
	}

	return nil
}

func (fbr *FilebeatRunner) hasRequiredFilebeatPaths(agentBasePath string) bool {
	curVerPath := filepath.Join(agentBasePath, currentVerLink)
	if _, err := os.Stat(curVerPath); os.IsNotExist(err) {
		log.WithField("path", curVerPath).Debug("missing current version link")
		return false
	}

	configsPath := filepath.Join(agentBasePath, configsSubpath)
	if _, err := os.Stat(configsPath); os.IsNotExist(err) {
		log.WithField("path", configsPath).Debug("missing configs path")
		return false
	}

	configsDir, err := os.Open(configsPath)
	if err != nil {
		log.WithError(err).Warn("unable to open configs directory for listing")
		return false
	}
	defer configsDir.Close()

	names, err := configsDir.Readdirnames(0)
	if err != nil {
		log.WithError(err).WithField("path", configsPath).
			Warn("unable to read files in configs directory")
		return false
	}

	for _, name := range names {
		if path.Ext(name) == ".yml" {
			return true
		}
	}
	log.WithField("path", configsPath).Debug("missing config files")
	return false
}
