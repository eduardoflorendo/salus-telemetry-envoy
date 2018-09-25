package agents

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	agentsSubpath  = "agents"
	configsSubpath = "config.d"
	currentVerLink = "CURRENT"
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

type AgentRunningInstance struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type AgentsRunner struct {
	DataPath       string
	LumberjackBind string

	running map[string]*AgentRunningInstance
}

func init() {
	viper.SetDefault("agents.dataPath", "data-telemetry-envoy")
}

func NewAgentsRunner() *AgentsRunner {
	ar := &AgentsRunner{
		DataPath:       viper.GetString("agents.dataPath"),
		LumberjackBind: viper.GetString("lumberjack.bind"),
		running:        make(map[string]*AgentRunningInstance),
	}

	return ar
}

func (ar *AgentsRunner) ensureFilebeatRunning() {

	log.Debug("ensuring filebeat is running")

	agentType := telemetry_edge.AgentType_FILEBEAT.String()

	if _, exists := ar.running[agentType]; exists {
		log.Debug("filebeat is already running")
		return
	}

	agentBasePath := filepath.Join(ar.DataPath, agentsSubpath, agentType)
	if !ar.hasRequiredFilebeatPaths(agentBasePath) {
		log.Debug("filebeat not ready to launch due to some missing paths and files")
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
		log.WithError(err).Warn("failed to start filebeat")
		return
	}

	runner := &AgentRunningInstance{
		ctx:    ctx,
		cancel: cancel,
		cmd:    cmd,
	}
	ar.running[agentType] = runner
	log.Debug("started filebeat")
}

func (ar *AgentsRunner) hasRequiredFilebeatPaths(agentBasePath string) bool {
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

func (ar *AgentsRunner) ProcessInstall(install *telemetry_edge.EnvoyInstructionInstall) {
	log.WithField("install", install).Debug("processing install instruction")

	agentType := install.Agent.Type.String()
	agentVersion := install.Agent.Version
	agentBasePath := path.Join(ar.DataPath, agentsSubpath, agentType)
	outputPath := path.Join(agentBasePath, agentVersion)

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		abs = outputPath
	}
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		err := os.MkdirAll(outputPath, 0755)
		if err != nil {
			log.WithError(err).WithField("path", outputPath).Error("unable to mkdirs")
			return
		}

		log.WithField("url", install.Url).Debug("downloading agent")
		resp, err := http.Get(install.Url)
		if err != nil {
			log.WithError(err).WithField("url", install.Url).Error("failed to download agent")
			return
		}

		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.WithError(err).Error("unable to ungzip agent download")
			return
		}
		defer resp.Body.Close()

		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}

			filename := header.Name
			stripped := filename[strings.Index(filename, "/")+1:]
			entryOutPath := path.Join(outputPath, stripped)
			if header.Typeflag&tar.TypeDir == tar.TypeDir {
				os.Mkdir(entryOutPath, os.FileMode(header.Mode))
			} else {
				file, err := os.OpenFile(entryOutPath, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
				if err != nil {
					log.WithError(err).Error("unable to open file for writing")
					continue
				}

				_, err = io.Copy(file, tarReader)
				if err != nil {
					file.Close()
					log.WithError(err).Error("unable to write to file")
					continue
				}
			}
		}

		currentSymlinkPath := path.Join(agentBasePath, currentVerLink)
		err = os.Remove(currentSymlinkPath)
		if err != nil && !os.IsNotExist(err) {
			log.WithError(err).Warn("failed to delete current version symlink")
		}
		err = os.Symlink(agentVersion, currentSymlinkPath)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"version": agentVersion,
				"type":    agentType,
			}).Error("failed to create current version symlink")
			return
		}

		ar.ensureFilebeatRunning()

		log.WithFields(log.Fields{
			"path":    abs,
			"type":    agentType,
			"version": agentVersion,
		}).Info("installed agent")
	} else {
		log.WithFields(log.Fields{
			"type":    agentType,
			"path":    abs,
			"version": agentVersion,
		}).Debug("agent already installed")

		ar.ensureFilebeatRunning()

	}
}

func (ar *AgentsRunner) ProcessConfigure(configure *telemetry_edge.EnvoyInstructionConfigure) {
	log.WithField("instruction", configure).Debug("processing configure instruction")

	switch configure.GetAgentType() {
	case telemetry_edge.AgentType_FILEBEAT:
		ar.processFilebeatConfig(configure)
	}

}

func (ar *AgentsRunner) processFilebeatConfig(configure *telemetry_edge.EnvoyInstructionConfigure) {
	agentTypeStr := telemetry_edge.AgentType_FILEBEAT.String()

	agentBasePath := path.Join(ar.DataPath, agentsSubpath, agentTypeStr)
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
		err = ar.createMainFilebeatConfig(agentBasePath, mainConfigPath)
		if err != nil {
			log.WithError(err).Error("failed to create main filebeat config")
			return
		}
	}

	for _, op := range configure.GetOperations() {
		log.WithField("op", op).Debug("processing filebeat config operation")

		err = ar.processFilebeatConfigOp(configsPath, op)
		if err != nil {
			log.WithField("op", op).Warn("failed to process filebeat config operation")
		}
	}

	ar.ensureFilebeatRunning()

}

func (ar *AgentsRunner) processFilebeatConfigOp(configsPath string, op *telemetry_edge.ConfigurationOp) error {
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

func (ar *AgentsRunner) createMainFilebeatConfig(agentBasePath, mainConfigPath string) error {

	log.WithField("path", mainConfigPath).Debug("creating main filebeat config file")

	_, port, err := net.SplitHostPort(ar.LumberjackBind)
	if err != nil {
		return errors.Wrapf(err, "unable to split lumberjack bind info: %v", ar.LumberjackBind)
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

func (ar *AgentsRunner) PurgeAgentConfigs() {
	configsPath := path.Join(ar.DataPath, agentsSubpath, telemetry_edge.AgentType_FILEBEAT.String(), configsSubpath)
	err := os.RemoveAll(configsPath)
	if err != nil {
		log.WithError(err).WithField("path", configsPath).Warn("failed to purge configs directory")
	}
}
