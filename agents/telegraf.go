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
	"fmt"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"text/template"
)

const (
	telegrafMainConfigFilename = "telegraf.conf"
)

var telegrafMainConfigTmpl = template.Must(template.New("telegrafMain").Parse(`
[agent]
  interval = "10s"
[[outputs.socket_writer]]
  address = "tcp://{{.IngestHost}}:{{.IngestPort}}"
  data_format = "json"
`))

type telegrafMainConfigData struct {
	IngestHost string
	IngestPort int
}

type TelegrafRunner struct {
	IngestHost string
	IngestPort int
	basePath   string
	running    *AgentRunningInstance
}

func init() {
	registerSpecificAgentRunner(telemetry_edge.AgentType_TELEGRAF, &TelegrafRunner{})
}

func (tr *TelegrafRunner) Load(agentBasePath string) error {
	tr.IngestHost = "localhost"
	tr.IngestPort = 8094
	tr.basePath = agentBasePath
	return nil
}

func (tr *TelegrafRunner) ProcessConfig(configure *telemetry_edge.EnvoyInstructionConfigure) error {
	configsPath := path.Join(tr.basePath, configsDirSubpath)
	err := os.MkdirAll(configsPath, dirPerms)
	if err != nil {
		return errors.Wrapf(err, "failed to create configs path for telegraf: %v", configsPath)
	}

	mainConfigPath := path.Join(tr.basePath, telegrafMainConfigFilename)
	if !fileExists(mainConfigPath) {
		err = tr.createMainConfig(mainConfigPath)
		if err != nil {
			return errors.Wrap(err, "failed to create main telegraf config")
		}
	}

	applied := 0
	for _, op := range configure.GetOperations() {
		log.WithField("op", op).Debug("processing telegraf config operation")

		configInstancePath := filepath.Join(configsPath, fmt.Sprintf("%s.conf", op.GetId()))

		switch op.GetType() {
		case telemetry_edge.ConfigurationOp_MODIFY:
			err := ioutil.WriteFile(configInstancePath, []byte(op.GetContent()), configFilePerms)
			if err != nil {
				log.WithField("op", op).Warn("failed to process telegraf config operation")
			} else {
				applied++
			}
		}
	}

	if applied > 0 {
		tr.handleConfigReload()
	}

	return nil
}

func (tr *TelegrafRunner) EnsureRunning(ctx context.Context) {
	log.Debug("ensuring telegraf is running")

	if tr.running.IsRunning() {
		log.Debug("telegraf is already running")
		return
	}

	if !tr.hasRequiredPaths() {
		log.Debug("telegraf not ready to launch due to some missing paths and files")
		return
	}

	cmdCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(cmdCtx,
		filepath.Join(currentVerLink, binSubpath, "telegraf"),
		"--config", telegrafMainConfigFilename,
		"--config-directory", configsDirSubpath,
		"--debug")
	cmd.Dir = tr.basePath

	err := startAgentCommand(cmdCtx, cmd, telemetry_edge.AgentType_TELEGRAF, "Agent Config:")
	if err != nil {
		log.WithError(err).
			WithField("agentType", telemetry_edge.AgentType_TELEGRAF).
			Warn("failed to start agent")
		cancel()
		return
	}

	go waitOnAgentCommand(ctx, tr, cmd)

	runner := &AgentRunningInstance{
		ctx:    cmdCtx,
		cancel: cancel,
		cmd:    cmd,
	}
	tr.running = runner
	log.WithField("pid", cmd.Process.Pid).
		WithField("agentType", telemetry_edge.AgentType_FILEBEAT).
		Info("started agent")
}

func (tr *TelegrafRunner) Stop() {
	if tr.running.IsRunning() {
		log.Debug("stopping telegraf")
		tr.running.cancel()
		tr.running = nil
	}
}

func (tr *TelegrafRunner) createMainConfig(mainConfigPath string) error {
	file, err := os.OpenFile(mainConfigPath, os.O_CREATE|os.O_RDWR, configFilePerms)
	if err != nil {
		return errors.Wrap(err, "unable to open main telegraf config file")
	}
	defer file.Close()

	data := &telegrafMainConfigData{
		IngestHost: tr.IngestHost,
		IngestPort: tr.IngestPort,
	}

	err = telegrafMainConfigTmpl.Execute(file, data)
	if err != nil {
		return errors.Wrap(err, "failed to execute telegraf main config template")
	}

	return nil
}

func (tr *TelegrafRunner) handleConfigReload() {
	if tr.running.IsRunning() {
		log.Debug("sending HUP signal to telegraf")
		tr.running.cmd.Process.Signal(syscall.SIGHUP)
	}
}

func (tr *TelegrafRunner) hasRequiredPaths() bool {

	mainConfigPath := path.Join(tr.basePath, telegrafMainConfigFilename)
	if !fileExists(mainConfigPath) {
		return false
	}

	configsPath := filepath.Join(tr.basePath, configsDirSubpath)
	if !fileExists(configsPath) {
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
		if path.Ext(name) == ".conf" {
			return true
		}
	}
	log.WithField("path", configsPath).Debug("missing config files")
	return false
}
