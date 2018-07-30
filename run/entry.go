package run

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
	"os"
	"runtime"
	"time"
)

type EnvoyRunner struct {
	config     *EnvoyRunnerConfig
	client     telemetry_edge.TelemetryAmbassadorClient
	instanceId string
	log        *zap.Logger
}

func NewEnvoyRunner(config *EnvoyRunnerConfig) (*EnvoyRunner, error) {
	log, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}

	return &EnvoyRunner{
		config: config,
		log:    log,
	}, nil
}

func (r *EnvoyRunner) Run() error {
	credsOption, err := loadTlsDialOption(r.config.CertPath, r.config.CaPath, r.config.KeyPath)
	if err != nil {
		return err
	}

	r.log.Info("using connection", zap.String("ambassador", r.config.AmbassadorAddress))
	conn, err := grpc.Dial(r.config.AmbassadorAddress, credsOption)
	if err != nil {
		return errors.Wrap(err, "failed to dial Ambassador")
	}
	defer conn.Close()

	r.client = telemetry_edge.NewTelemetryAmbassadorClient(conn)
	r.instanceId = uuid.NewV1().String()

	rootCtx := context.Background()

	for {
		backoff.RetryNotify(r.attach, backoff.WithContext(backoff.NewExponentialBackOff(), rootCtx),
			func(err error, delay time.Duration) {
				r.log.Warn("delaying until next attempt",
					zap.Error(err), zap.Duration("delay", delay))
			})

		r.instanceId = uuid.NewV1().String()
	}
}

func (r *EnvoyRunner) sendKeepAlives(ctx context.Context, errChan chan<- error) {
	for {
		select {
		case <-time.After(r.config.KeepAliveInterval):
			_, err := r.client.KeepAlive(ctx, &telemetry_edge.KeepAliveRequest{
				InstanceId: r.instanceId,
			})
			if err != nil {
				errChan <- errors.Wrap(err, "failed to send keep alive")
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (r *EnvoyRunner) attach() error {
	ctx, cancelFunc := context.WithCancel(context.Background())

	envoySummary := &telemetry_edge.EnvoySummary{
		InstanceId:      r.instanceId,
		SupportedAgents: []telemetry_edge.AgentType{telemetry_edge.AgentType_FILEBEAT},
		Labels:          r.computeLabels(),
	}
	r.log.Info("attaching", zap.Any("summary", envoySummary))

	instructions, err := r.client.AttachEnvoy(ctx, envoySummary)
	if err != nil {
		return errors.Wrap(err, "failed to attach Envoy")
	}

	errChan := make(chan error, 10)

	go r.watchForInstructions(ctx, errChan, instructions)
	go r.sendKeepAlives(ctx, errChan)
	go r.handleLumberjack(ctx, errChan)

	for {
		select {
		case <-ctx.Done():
			instructions.CloseSend()
			return nil

		case err := <-errChan:
			r.log.Warn("terminating", zap.Error(err))
			cancelFunc()
		}
	}
}

func (r *EnvoyRunner) computeLabels() map[string]string {
	labels := make(map[string]string)

	labels["os"] = runtime.GOOS
	labels["arch"] = runtime.GOARCH

	hostname, err := os.Hostname()
	if err == nil {
		labels["hostname"] = hostname
	} else {
		r.log.Warn("unable to determine hostname", zap.Error(err))
	}

	return labels
}

func loadTlsDialOption(CertPath, CaPath, KeyPath string) (grpc.DialOption, error) {
	// load ours
	certificate, err := tls.LoadX509KeyPair(
		CertPath,
		KeyPath,
	)

	// load the CA
	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(CaPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read ca cert")
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, errors.Wrap(err, "failed to append certs")
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	})
	return grpc.WithTransportCredentials(transportCreds), nil
}
