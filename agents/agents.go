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
	"time"
)

const (
	agentsSubpath     = "agents"
	configsDirSubpath = "config.d"
	currentVerLink    = "CURRENT"
	binSubpath        = "bin"
	dirPerms          = 0755
	configFilePerms   = 0600
)

var (
	agentRestartDelay = 1 * time.Second
)

type SpecificAgentRunner interface {
	// Load gets called after viper's configuration has been populated and before any other use.
	Load(agentBasePath string) error
	SetCommandHandler(handler CommandHandler)
	// EnsureRunning after installation of an agent and each call to ProcessConfig
	EnsureRunning(ctx context.Context)
	ProcessConfig(configure *telemetry_edge.EnvoyInstructionConfigure) error
	// Stop should stop the agent's process, if running
	Stop()
}

type AgentsRunner interface {
	Start(ctx context.Context)
	ProcessInstall(install *telemetry_edge.EnvoyInstructionInstall)
	ProcessConfigure(configure *telemetry_edge.EnvoyInstructionConfigure)
}

type AgentRunningInstance struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

func init() {
	viper.SetDefault("agents.dataPath", "data-telemetry-envoy")
}

func downloadExtractTarGz(outputPath, url string, exePath string) error {

	log.WithField("file", url).Debug("downloading agent")
	resp, err := http.Get(url)
	if err != nil {
		return errors.Wrap(err, "failed to download agent")
	}
	defer resp.Body.Close()

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return errors.Wrap(err, "unable to ungzip agent download")
	}

	_, exeFilename := path.Split(exePath)
	binOutPath := path.Join(outputPath, binSubpath)
	err = os.Mkdir(binOutPath, dirPerms)
	if err != nil {
		return errors.Wrap(err, "unable to create bin directory")
	}

	processedExe := false
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if header.Name == exePath {

			file, err := os.OpenFile(path.Join(binOutPath, exeFilename), os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
			if err != nil {
				log.WithError(err).Error("unable to open file for writing")
				continue
			}

			_, err = io.Copy(file, tarReader)
			if err != nil {
				file.Close()
				log.WithError(err).Error("unable to write to file")
				continue
			} else {
				processedExe = true
			}
		}
	}

	if !processedExe {
		return errors.New("failed to find/process agent executable")
	}

	return nil
}

func (inst *AgentRunningInstance) IsRunning() bool {
	return inst != nil && inst.cmd != nil && (inst.cmd.ProcessState == nil || !inst.cmd.ProcessState.Exited())
}

func fileExists(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	} else if err != nil {
		log.WithError(err).Warn("failed to stat file")
		return false
	} else {
		return true
	}
}
