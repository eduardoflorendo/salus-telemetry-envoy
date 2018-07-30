package run

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"go.uber.org/zap"
	"io"
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

			r.log.Debug("received instruction", zap.Any("instruction", instruction))

			install := instruction.GetInstall()

			switch {
			case install != nil:
				r.processInstall(install)

			}
		}
	}
}

func (r *EnvoyRunner) processInstall(install *telemetry_edge.EnvoyInstructionInstall) {
	r.log.Debug("processing install instruction", zap.Any("install", install))

	agentType := install.Agent.Type.String()
	agentVersion := install.Agent.Version
	outputPath := path.Join(r.config.BinPath, agentType, agentVersion)

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

		r.log.Info("installed agent", zap.String("path", abs),
			zap.String("type", agentType), zap.String("version", agentVersion))
	} else {
		r.log.Debug("agent already installed", zap.String("path", abs),
			zap.String("type", agentType), zap.String("version", agentVersion))
	}
}
