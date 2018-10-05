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
	"bufio"
	"context"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	log "github.com/sirupsen/logrus"
	"io"
	"os/exec"
	"strings"
	"time"
)

type StandardCommandHandler struct{}

func NewCommandHandler() CommandHandler {
	return &StandardCommandHandler{}
}

func (h *StandardCommandHandler) StartAgentCommand(cmdCtx context.Context, cmd *exec.Cmd, agentType telemetry_edge.AgentType, waitFor string, waitForDuration time.Duration) error {
	waitForChan, err := h.setupCommandLogging(cmdCtx, agentType, cmd, waitFor)
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
	case <-time.After(waitForDuration):
		return errors.New("failed to see expected content")
	case <-waitForChan:
		return nil
	}
}

func (*StandardCommandHandler) WaitOnAgentCommand(ctx context.Context, agentRunner SpecificAgentRunner, cmd *exec.Cmd) {
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

func (h *StandardCommandHandler) setupCommandLogging(ctx context.Context, agentType telemetry_edge.AgentType, cmd *exec.Cmd, waitFor string) (<-chan struct{}, error) {
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

	go h.handleCommandOutputPipe(ctx, "stdout", stdoutPipe, agentType, waitFor, waitForChan)
	go h.handleCommandOutputPipe(ctx, "stderr", stderrPipe, agentType, waitFor, waitForChan)

	return waitForChan, nil
}

func (*StandardCommandHandler) handleCommandOutputPipe(ctx context.Context, outputType string, stdoutPipe io.ReadCloser, agentType telemetry_edge.AgentType, waitFor string, waitForChan chan struct{}) {
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
