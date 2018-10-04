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
	"bufio"
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
	"strings"
	"time"
)

const (
	agentsSubpath     = "agents"
	configsDirSubpath = "config.d"
	currentVerLink    = "CURRENT"
	binSubpath        = "bin"
	dirPerms          = 0755
	configFilePerms   = 0600
	agentRestartDelay = 1 * time.Second
)

type SpecificAgentRunner interface {
	// Load gets called after viper's configuration has been populated and before any other use.
	Load(agentBasePath string) error
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

func init() {
	viper.SetDefault("agents.dataPath", "data-telemetry-envoy")
}

func downloadExtractTarGz(outputPath, url string, exePath string) error {

	log.WithField("url", url).Debug("downloading agent")
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

func setupCommandLogging(ctx context.Context, agentType telemetry_edge.AgentType, cmd *exec.Cmd, waitFor string) (<-chan struct{}, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	waitForChan := make(chan struct{}, 1)
	if waitFor == "" {
		close(waitForChan)
	}

	go handleCommandOutputPipe(ctx, "stdout", stdoutPipe, agentType, waitFor, waitForChan)
	go handleCommandOutputPipe(ctx, "stderr", stderrPipe, agentType, waitFor, waitForChan)

	return waitForChan, nil
}

func handleCommandOutputPipe(ctx context.Context, outputType string, stdoutPipe io.ReadCloser, agentType telemetry_edge.AgentType, waitFor string, waitForChan chan struct{}) {
	stdoutReader := bufio.NewReader(stdoutPipe)
	defer stdoutPipe.Close()
	defer log.
		WithField("agentType", agentType).
		Debugf("stopping command %s forwarding to logs", outputType)

	checkingWaitFor := waitFor != ""
	for {
		select {
		case <-ctx.Done():
			return

		default:
			line, err := stdoutReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.WithError(err).WithField("agentType", agentType).Warnf("while reading command's %s", outputType)
				}
				return
			}
			log.WithField("agentType", agentType).Info(line)

			if checkingWaitFor && strings.Contains(line, waitFor) {
				log.WithField("agentType", agentType).Debug("saw expected content")
				close(waitForChan)
				checkingWaitFor = false
			}
		}
	}
}

func waitOnAgentCommand(ctx context.Context, agentRunner SpecificAgentRunner, cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.WithError(err).
			WithField("agentType", telemetry_edge.AgentType_FILEBEAT).
			Warn("agent exited abnormally")
	} else {
		log.
			WithField("agentType", telemetry_edge.AgentType_FILEBEAT).
			Info("agent exited successfully")
	}

	agentRunner.Stop()
	log.
		WithField("agentType", telemetry_edge.AgentType_FILEBEAT).
		Info("scheduling agent restart")
	time.AfterFunc(agentRestartDelay, func() {
		agentRunner.EnsureRunning(ctx)
	})
}

func startAgentCommand(cmdCtx context.Context, cmd *exec.Cmd, agentType telemetry_edge.AgentType, waitFor string) error {
	waitForChan, err := setupCommandLogging(cmdCtx, agentType, cmd, waitFor)
	if err != nil {
		return errors.New("logging and watching command output")
	}

	log.
		WithField("agentType", agentType).
		WithField("cmd", cmd).
		Debug("starting agent")
	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start command")
	}

	select {
	case <-time.After(5 * time.Second):
		return errors.New("failed to see expected content")
	case <-waitForChan:
		return nil
	}
}
