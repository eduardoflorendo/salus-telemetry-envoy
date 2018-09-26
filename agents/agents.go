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

package agents

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"github.com/pkg/errors"
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
	// Load gets called after viper's configuration has been populated and before any other use.
	Load() error
	// EnsureRunning after installation of an agent and each call to ProcessConfig
	EnsureRunning(ctx context.Context)
	ProcessConfig(configure *telemetry_edge.EnvoyInstructionConfigure) error
	// Stop should stop the agent's process, if running
	Stop()
}

type AgentRunningInstance struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type AgentsRunner struct {
	DataPath string

	ctx context.Context
}

var specificAgentRunners = make(map[string]SpecificAgentRunner)

func registerSpecificAgentRunner(agentType telemetry_edge.AgentType, runner SpecificAgentRunner) {
	specificAgentRunners[agentType.String()] = runner
}

func init() {
	viper.SetDefault("agents.dataPath", "data-telemetry-envoy")
}

func NewAgentsRunner() (*AgentsRunner, error) {
	ar := &AgentsRunner{
		DataPath: viper.GetString("agents.dataPath"),
	}

	for _, runner := range specificAgentRunners {
		err := runner.Load()
		if err != nil {
			return nil, errors.Wrapf(err, "loading agent runner: %T", runner)
		}
	}

	return ar, nil
}

func (ar *AgentsRunner) Start(ctx context.Context) {
	ar.ctx = ctx

	for {
		select {
		case <-ar.ctx.Done():
			log.Debug("stopping specific runners")
			for _, specific := range specificAgentRunners {
				specific.Stop()
			}
			return
		}
	}
}

func (ar *AgentsRunner) ProcessInstall(install *telemetry_edge.EnvoyInstructionInstall) {
	log.WithField("install", install).Debug("processing install instruction")

	agentType := install.Agent.Type.String()
	if _, exists := specificAgentRunners[agentType]; !exists {
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

		specificAgentRunners[agentType].EnsureRunning(ar.ctx)

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

		specificAgentRunners[agentType].EnsureRunning(ar.ctx)

	}
}

func (ar *AgentsRunner) ProcessConfigure(configure *telemetry_edge.EnvoyInstructionConfigure) {
	log.WithField("instruction", configure).Debug("processing configure instruction")

	if specificRunner, exists := specificAgentRunners[configure.GetAgentType().String()]; exists {
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
	for agentType, _ := range specificAgentRunners {
		configsPath := path.Join(ar.DataPath, agentsSubpath, agentType, configsSubpath)
		log.WithField("path", configsPath).Debug("purging agent config directory")
		err := os.RemoveAll(configsPath)
		if err != nil {
			log.WithError(err).WithField("path", configsPath).Warn("failed to purge configs directory")
		}
	}
}
