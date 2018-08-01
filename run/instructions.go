package run

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (r *EnvoyRunner) watchForInstructions(ctx context.Context,
	errChan chan<- error, instructions telemetry_edge.TelemetryAmbassador_AttachEnvoyClient) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			instruction, err := instructions.Recv()
			if err != nil {
				errChan <- errors.Wrap(err, "failed to receive instruction")
				return
			}

			switch {
			case instruction.GetInstall() != nil:
				r.processInstall(instruction.GetInstall())

			case instruction.GetConfigure() != nil:
				r.processConfigure(instruction.GetConfigure())
			}
		}
	}
}

func (r *EnvoyRunner) processInstall(install *telemetry_edge.EnvoyInstructionInstall) {
	r.log.Debug("processing install instruction", zap.Any("install", install))

	agentType := install.Agent.Type.String()
	agentVersion := install.Agent.Version
	agentBasePath := path.Join(r.config.DataPath, agentsSubpath, agentType)
	outputPath := path.Join(agentBasePath, agentVersion)

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		abs = outputPath
	}
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		err := os.MkdirAll(outputPath, 0755)
		if err != nil {
			r.log.Error("unable to mkdirs", zap.Error(err), zap.String("path", outputPath))
			return
		}

		r.log.Debug("downloading agent", zap.String("url", install.Url))
		resp, err := http.Get(install.Url)
		if err != nil {
			r.log.Error("failed to download agent", zap.Error(err), zap.String("url", install.Url))
			return
		}

		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			r.log.Error("unable to ungzip agent download", zap.Error(err))
			return
		}
		defer resp.Body.Close()

		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}

			filename := header.Name
			stripped := filename[strings.Index(filename, "/")+1:]
			entryOutPath := path.Join(outputPath, stripped)
			if header.Typeflag&tar.TypeDir == tar.TypeDir {
				os.Mkdir(entryOutPath, os.FileMode(header.Mode))
			} else {
				file, err := os.OpenFile(entryOutPath, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
				if err != nil {
					r.log.Error("unable to open file for writing", zap.Error(err))
					continue
				}

				_, err = io.Copy(file, tarReader)
				if err != nil {
					file.Close()
					r.log.Error("unable to write to file", zap.Error(err))
					continue
				}
			}
		}

		err = os.Symlink(agentVersion, path.Join(agentBasePath, currentVerLink))
		if err != nil {
			r.log.Error("failed to create current version symlink",
				zap.Error(err), zap.String("version", agentVersion), zap.String("type", agentType))
			return
		}

		r.agents.ensureFilebeatRunning()

		r.log.Info("installed agent", zap.String("path", abs),
			zap.String("type", agentType), zap.String("version", agentVersion))
	} else {
		r.log.Debug("agent already installed", zap.String("path", abs),
			zap.String("type", agentType), zap.String("version", agentVersion))

		r.agents.ensureFilebeatRunning()

	}
}

func (r *EnvoyRunner) processConfigure(configure *telemetry_edge.EnvoyInstructionConfigure) {
	r.log.Debug("processing configure instruction", zap.Any("instruction", configure))

	switch configure.GetAgentType() {
	case telemetry_edge.AgentType_FILEBEAT:
		r.processFilebeatConfig(configure)
	}

}

func (r *EnvoyRunner) processFilebeatConfig(configure *telemetry_edge.EnvoyInstructionConfigure) {
	agentTypeStr := telemetry_edge.AgentType_FILEBEAT.String()

	agentBasePath := path.Join(r.config.DataPath, agentsSubpath, agentTypeStr)
	err := os.MkdirAll(agentBasePath, 0755)
	if err != nil {
		r.log.Error("failed to create agent base path",
			zap.Error(err), zap.String("path", agentBasePath))
		return
	}

	configsPath := path.Join(agentBasePath, configsSubpath)
	err = os.MkdirAll(configsPath, 0755)
	if err != nil {
		r.log.Error("failed to create configs path for filebeat",
			zap.Error(err), zap.String("path", configsPath))
		return
	}

	mainConfigPath := path.Join(agentBasePath, filebeatMainConfigFilename)
	if _, err := os.Stat(mainConfigPath); os.IsNotExist(err) {
		err = r.createMainFilebeatConfig(agentBasePath, mainConfigPath)
		if err != nil {
			r.log.Error("failed to create main filebeat config",
				zap.Error(err))
			return
		}
	}

	for _, op := range configure.GetOperations() {
		r.log.Debug("processing filebeat config operation", zap.Any("op", op))

		err = r.processFilebeatConfigOp(configsPath, op)
		if err != nil {
			r.log.Warn("failed to process filebeat config operation", zap.Any("op", op))
		}
	}

	r.agents.ensureFilebeatRunning()

}

func (r *EnvoyRunner) processFilebeatConfigOp(configsPath string, op *telemetry_edge.ConfigurationOp) error {
	configInstancePath := filepath.Join(configsPath, fmt.Sprintf("%s.yml", op.GetId()))

	switch op.GetType() {
	case telemetry_edge.ConfigurationOp_MODIFY:
		err := ioutil.WriteFile(configInstancePath, []byte(op.GetContent()), 0644)
		if err != nil {
			return errors.Wrap(err, "failed to write filebeat config file instance")
		}
	}

	return nil
}

func (r *EnvoyRunner) createMainFilebeatConfig(agentBasePath, mainConfigPath string) error {

	r.log.Debug("creating main filebeat config file",
		zap.String("path", mainConfigPath))

	file, err := os.OpenFile(mainConfigPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return errors.Wrap(err, "unable to open main filebeat config file")
	}
	defer file.Close()

	_, port, err := net.SplitHostPort(r.config.LumberjackBind)
	if err != nil {
		return errors.Wrap(err, "unable to split lumberjack bind info")
	}

	data := filebeatMainConfigData{
		ConfigsPath:    configsSubpath,
		LumberjackPort: port,
	}

	err = filebeatMainConfigTmpl.Execute(file, data)
	if err != nil {
		return errors.Wrap(err, "failed to execute filebeat main config template")
	}

	return nil
}
