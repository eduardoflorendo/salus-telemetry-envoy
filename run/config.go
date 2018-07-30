package run

import (
	"time"
)

type EnvoyRunnerConfig struct {
	KeepAliveInterval time.Duration
	AmbassadorAddress string
	CertPath          string
	CaPath            string
	KeyPath           string
	BinPath           string
	LumberjackBind	string
}

func NewEnvoyRunnerConfig() *EnvoyRunnerConfig {
	config := &EnvoyRunnerConfig{
		KeepAliveInterval: 10 * time.Second,
		AmbassadorAddress: "localhost:6565",
		BinPath:           "agents",
		LumberjackBind: "localhost:5044",
	}

	return config
}
