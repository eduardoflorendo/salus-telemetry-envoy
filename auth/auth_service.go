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

package auth

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type AuthServiceCertProvider struct{}

type authServiceResponse struct {
	Certificate          string `json:"certificate"`
	PrivateKey           string `json:"privateKey"`
	IssuingCACertificate string `json:"issuingCaCertificate"`
}

func (p *AuthServiceCertProvider) ProvideCertificates(config *TlsConfig) (*tls.Certificate, *x509.CertPool, error) {

	log.WithField("config", config.AuthService).Debug("acquiring certificates from auth service")

	provider, err := GetAuthTokenProvider(config.AuthService.TokenProvider)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get AuthTokenProvider")
	}

	token, err := provider.ProvideAuthToken()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get auth token")
	}

	fullUrl, err := AppendUrlPath(config.AuthService.Url, "auth/cert")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to build request url")
	}

	request, err := http.NewRequest("GET", fullUrl, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to prepare auth service request")
	}

	for header, value := range token.Headers {
		request.Header.Set(header, value)
	}
	request.Header.Set("Accept", "application/json")

	client := &http.Client{}
	httpResp, err := client.Do(request)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failure during auth service request")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		return nil, nil, errors.Errorf("http request to auth service failed: %s", httpResp.Status)
	}

	var resp authServiceResponse
	decoder := json.NewDecoder(httpResp.Body)
	err = decoder.Decode(&resp)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to decode auth service response")
	}

	if resp.Certificate == "" || resp.PrivateKey == "" || resp.IssuingCACertificate == "" {
		return nil, nil, errors.Errorf("auth service response was missing a required field: cert=%t, key=%t, ca=%t",
			resp.Certificate != "", resp.PrivateKey != "", resp.IssuingCACertificate != "")
	}

	return p.loadFromResponse(resp)
}

func (p *AuthServiceCertProvider) loadFromResponse(response authServiceResponse) (*tls.Certificate, *x509.CertPool, error) {
	certificate, err := tls.X509KeyPair([]byte(response.Certificate), []byte(response.PrivateKey))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load certificates")
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM([]byte(response.IssuingCACertificate))
	if !ok {
		return nil, nil, errors.New("failed to process CA cert")
	}

	log.Info("successfully acquired certificates from auth service")
	return &certificate, certPool, nil
}
