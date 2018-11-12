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
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"os"
	"os/exec"
	"time"
)

// NOTE this file is specifically declared in the agents package, but only compiled during testing due
// to the file name. As such, it is used to enable unit testing access to package-private aspects

func RegisterAgentRunnerForTesting(agentType telemetry_edge.AgentType, runner SpecificAgentRunner) {
	registerSpecificAgentRunner(agentType, runner)
}

func UnregisterAllAgentRunners() {
	specificAgentRunners = make(map[telemetry_edge.AgentType]SpecificAgentRunner)
}

func SetAgentRestartDelay(delay time.Duration) {
	agentRestartDelay = delay
}

func RunAgentRunningContext(ctx *AgentRunningContext) error {
	return ctx.cmd.Run()
}

func CreateNoAppliedConfigsError() error {
	return &noAppliedConfigsError{}
}

func CreatePreRunningAgentRunningContext() *AgentRunningContext {
	return &AgentRunningContext{
		cmd: &exec.Cmd{
			Process: &os.Process{},
		},
	}
}
