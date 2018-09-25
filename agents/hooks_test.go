package agents

import "github.com/racker/telemetry-envoy/telemetry_edge"

func InjectMockRunnersIntoAgentRunner(runner *AgentsRunner, agentRunner SpecificAgentRunner) {
	runner.specificRunners[telemetry_edge.AgentType_FILEBEAT.String()] = agentRunner
}
