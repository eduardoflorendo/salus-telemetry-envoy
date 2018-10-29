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
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func init() {
	RegisterAuthTokenProvider("static", NewStaticAuthTokenProvider)
}

type StaticAuthTokenConfig struct {
	Headers []struct {
		Name  string
		Value string
	}
}

type StaticAuthTokenProvider struct {
	config *StaticAuthTokenConfig
}

func NewStaticAuthTokenProvider() (AuthTokenProvider, error) {
	var config StaticAuthTokenConfig
	err := viper.UnmarshalKey("tls.token_providers.static", &config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to process static token provider config")
	}

	return &StaticAuthTokenProvider{config: &config}, nil
}

func (p *StaticAuthTokenProvider) ProvideAuthToken() (*AuthToken, error) {
	// NOTE: the config field couldn't be a map[string]string since viper normalizes config keys to lowercase
	headers := make(map[string]string)
	for _, entry := range p.config.Headers {
		headers[entry.Name] = entry.Value
	}
	return &AuthToken{Headers: headers}, nil
}
