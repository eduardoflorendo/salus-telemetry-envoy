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
	"context"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"path"
	"path/filepath"
)

type AgentsRunner struct {
	DataPath string

	ctx context.Context
}

func NewAgentsRunner() (*AgentsRunner, error) {
	ar := &AgentsRunner{
		DataPath: viper.GetString("agents.dataPath"),
	}

	for agentType, runner := range specificAgentRunners {

		agentBasePath := filepath.Join(ar.DataPath, agentsSubpath, agentType.String())

		err := runner.Load(agentBasePath)
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

	agentType := install.Agent.Type
	if _, exists := specificAgentRunners[agentType]; !exists {
		log.WithField("type", agentType).Warn("no specific runner for agent type")
		return
	}

	agentVersion := install.Agent.Version
	agentBasePath := path.Join(ar.DataPath, agentsSubpath, agentType.String())
	outputPath := path.Join(agentBasePath, agentVersion)

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		abs = outputPath
	}
	if !fileExists(outputPath) {
		err := os.MkdirAll(outputPath, dirPerms)
		if err != nil {
			log.WithError(err).WithField("path", outputPath).Error("unable to mkdirs")
			return
		}

		err = downloadExtractTarGz(outputPath, install.Url, install.Exe)
		if err != nil {
			os.RemoveAll(outputPath)
			log.WithError(err).Error("failed to download and extract agent")
			return
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

	agentType := configure.GetAgentType()
	if specificRunner, exists := specificAgentRunners[agentType]; exists {

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
		configsPath := path.Join(ar.DataPath, agentsSubpath, agentType.String(), configsDirSubpath)
		log.WithField("path", configsPath).Debug("purging agent config directory")
		err := os.RemoveAll(configsPath)
		if err != nil {
			log.WithError(err).WithField("path", configsPath).Warn("failed to purge configs directory")
		}
	}
}
