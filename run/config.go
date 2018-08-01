package run

import (
	"text/template"
	"time"
)

type EnvoyRunnerConfig struct {
	KeepAliveInterval time.Duration
	AmbassadorAddress string
	CertPath          string
	CaPath            string
	KeyPath           string
	DataPath          string
	LumberjackBind    string
	GrpcCallLimit     time.Duration
}

var envoyRunnerConfig = &EnvoyRunnerConfig{
	KeepAliveInterval: 10 * time.Second,
	AmbassadorAddress: "localhost:6565",
	DataPath:          "data-telemetry-envoy",
	LumberjackBind:    "localhost:5044",
	GrpcCallLimit:     5 * time.Second,
}

const (
	agentsSubpath  = "agents"
	configsSubpath = "config.d"
	currentVerLink = "CURRENT"
)

const (
	filebeatMainConfigFilename = "filebeat.yml"
)

var filebeatMainConfigTmpl = template.Must(template.New("filebeatMain").Parse(`
filebeat.config.inputs:
  enabled: true
  path: {{.ConfigsPath}}/*.yml
  reload.enabled: true
  reload.period: 5s
output.logstash:
  hosts: ["localhost:{{.LumberjackPort}}"]
`))

type filebeatMainConfigData struct {
	ConfigsPath    string
	LumberjackPort string
}
