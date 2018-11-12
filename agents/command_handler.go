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
	"syscall"
	"time"
)

// CommandHandler abstracts access to an agent's process and managing its lifecycle and output
type CommandHandler interface {
	CreateContext(ctx context.Context, agentType telemetry_edge.AgentType, cmdName string, workingDir string, arg ...string) *AgentRunningContext
	// StartAgentCommand will start the given command and optionally block until a specific phrase
	// is observed as given by the waitFor argument.
	// It will also setup "forwarding" of the command's stdout/err to logrus
	StartAgentCommand(runningContext *AgentRunningContext, agentType telemetry_edge.AgentType, waitFor string, waitForDuration time.Duration) error
	// WaitOnAgentCommand should be ran as a goroutine to watch for the agent process to end prematurely.
	// It will take care of restarting the agent via the SpecificAgentRunner's EnsureRunningState function.
	WaitOnAgentCommand(ctx context.Context, agentRunner SpecificAgentRunner, runningContext *AgentRunningContext)
	Signal(runningContext *AgentRunningContext, signal syscall.Signal) error
	Stop(runningContext *AgentRunningContext)
}

type StandardCommandHandler struct{}

func NewCommandHandler() CommandHandler {
	return &StandardCommandHandler{}
}

func (h *StandardCommandHandler) CreateContext(ctx context.Context, agentType telemetry_edge.AgentType, cmdName string, workingDir string, arg ...string) *AgentRunningContext {
	cmdCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(cmdCtx, cmdName, arg...)
	cmd.Dir = workingDir

	return &AgentRunningContext{
		agentType: agentType,
		ctx:       cmdCtx,
		cancel:    cancel,
		cmd:       cmd,
	}
}

func (h *StandardCommandHandler) StartAgentCommand(runningContext *AgentRunningContext, agentType telemetry_edge.AgentType, waitFor string, waitForDuration time.Duration) error {
	cmdCtx := runningContext.ctx
	cmd := runningContext.cmd

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
	case result := <-waitForChan:
		if result {
			return nil
		} else {
			return errors.New("command exited before seeing expected content")
		}
	}
}

func (h *StandardCommandHandler) WaitOnAgentCommand(ctx context.Context, agentRunner SpecificAgentRunner, runningContext *AgentRunningContext) {
	err := runningContext.cmd.Wait()
	if err != nil {
		log.WithError(err).
			WithField("agentType", telemetry_edge.AgentType_FILEBEAT).
			Warn("agent exited abnormally")
	} else {
		log.
			WithField("agentType", telemetry_edge.AgentType_FILEBEAT).
			Info("agent exited successfully")
	}

	h.Stop(runningContext)
	log.
		WithField("agentType", telemetry_edge.AgentType_FILEBEAT).
		Info("scheduling agent restart")
	time.AfterFunc(agentRestartDelay, func() {
		agentRunner.EnsureRunningState(ctx, false)
	})
}

func (h *StandardCommandHandler) Signal(runningContext *AgentRunningContext, signal syscall.Signal) error {
	if runningContext.IsRunning() {
		log.WithField("agentType", runningContext.agentType).Debug("sending HUP signal to agent")

		return runningContext.cmd.Process.Signal(signal)
	} else {
		return nil
	}
}

func (h *StandardCommandHandler) setupCommandLogging(ctx context.Context, agentType telemetry_edge.AgentType, cmd *exec.Cmd, waitFor string) (<-chan bool, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	waitForChan := make(chan bool, 1)
	if waitFor == "" {
		waitForChan <- true
	}

	go h.handleCommandOutputPipe(ctx, "stdout", stdoutPipe, agentType, waitFor, waitForChan)
	go h.handleCommandOutputPipe(ctx, "stderr", stderrPipe, agentType, waitFor, waitForChan)

	return waitForChan, nil
}

func (*StandardCommandHandler) handleCommandOutputPipe(ctx context.Context, outputType string, stdoutPipe io.ReadCloser, agentType telemetry_edge.AgentType, waitFor string, waitForChan chan bool) {
	stdoutReader := bufio.NewReader(stdoutPipe)
	//noinspection GoUnhandledErrorResult
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
				waitForChan <- false
				return
			}
			log.WithField("agentType", agentType).Info(line)

			if checkingWaitFor && strings.Contains(line, waitFor) {
				log.WithField("agentType", agentType).Debug("saw expected content")
				waitForChan <- true
				checkingWaitFor = false
			}
		}
	}
}

func (*StandardCommandHandler) Stop(runningContext *AgentRunningContext) {
	if runningContext.IsRunning() {
		log.WithField("agentType", runningContext.agentType).Debug("stopping agent")
		runningContext.cancel()
	}
}

// AgentRunningContext encapsulates the state of a running agent process
// This should be created using CommandHandler's CreateContext
type AgentRunningContext struct {
	agentType telemetry_edge.AgentType
	ctx       context.Context
	cancel    context.CancelFunc
	cmd       *exec.Cmd
}

func (c *AgentRunningContext) IsRunning() bool {
	return c != nil && c.cmd != nil && (c.cmd.ProcessState == nil || !c.cmd.ProcessState.Exited())
}

func (c *AgentRunningContext) Pid() int {
	if c.IsRunning() {
		return c.cmd.Process.Pid
	} else {
		return -1
	}
}
