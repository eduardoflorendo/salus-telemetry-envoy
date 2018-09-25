package agents

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const (
	agentsSubpath  = "agents"
	configsSubpath = "config.d"
	currentVerLink = "CURRENT"
)

type SpecificAgentRunner interface {
	EnsureRunning(ctx context.Context)
	ProcessConfig(configure *telemetry_edge.EnvoyInstructionConfigure) error
	Stop()
}

type AgentRunningInstance struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type AgentsRunner struct {
	DataPath string

	specificRunners map[string]SpecificAgentRunner
	ctx             context.Context
}

func init() {
	viper.SetDefault("agents.dataPath", "data-telemetry-envoy")
}

func NewAgentsRunner() *AgentsRunner {
	ar := &AgentsRunner{
		DataPath:        viper.GetString("agents.dataPath"),
		specificRunners: make(map[string]SpecificAgentRunner),
	}

	ar.specificRunners[telemetry_edge.AgentType_FILEBEAT.String()] = NewFilebeatRunner()

	return ar
}

func (ar *AgentsRunner) Start(ctx context.Context) {
	ar.ctx = ctx

	for {
		select {
		case <-ar.ctx.Done():
			log.Debug("stopping specific runners")
			for _, specific := range ar.specificRunners {
				specific.Stop()
			}
			return
		}
	}
}

func (ar *AgentsRunner) ProcessInstall(install *telemetry_edge.EnvoyInstructionInstall) {
	log.WithField("install", install).Debug("processing install instruction")

	agentType := install.Agent.Type.String()
	if _, exists := ar.specificRunners[agentType]; !exists {
		log.WithField("type", agentType).Warn("no specific runner for agent type")
		return
	}

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

		ar.specificRunners[agentType].EnsureRunning(ar.ctx)

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

		ar.specificRunners[agentType].EnsureRunning(ar.ctx)

	}
}

func (ar *AgentsRunner) ProcessConfigure(configure *telemetry_edge.EnvoyInstructionConfigure) {
	log.WithField("instruction", configure).Debug("processing configure instruction")

	if specificRunner, exists := ar.specificRunners[configure.GetAgentType().String()]; exists {
		err := specificRunner.ProcessConfig(configure)
		if err != nil {
			log.WithError(err).Warn("failed to process agent configuration")
		} else {
			specificRunner.EnsureRunning(ar.ctx)
		}
	} else {
		log.WithField("type", configure.GetAgentType()).Warn("unable to configure unknown agent type")
	}
}

func (ar *AgentsRunner) PurgeAgentConfigs() {
	for agentType, _ := range ar.specificRunners {
		configsPath := path.Join(ar.DataPath, agentsSubpath, agentType, configsSubpath)
		log.WithField("path", configsPath).Debug("purging agent config directory")
		err := os.RemoveAll(configsPath)
		if err != nil {
			log.WithError(err).WithField("path", configsPath).Warn("failed to purge configs directory")
		}
	}
}
