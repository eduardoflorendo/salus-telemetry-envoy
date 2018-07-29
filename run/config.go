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
}

func LoadEnvoyRunnerConfig() *EnvoyRunnerConfig {
	config := &EnvoyRunnerConfig{
		KeepAliveInterval: 10 * time.Second,
		AmbassadorAddress: "localhost:6565",
		BinPath:           "agents",
	}

	return config
}
