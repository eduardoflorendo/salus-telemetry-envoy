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

import "github.com/pkg/errors"

type AuthToken struct {
	Header string
	Value  string
}

type AuthTokenProvider interface {
	ProvideAuthToken() (*AuthToken, error)
}

type AuthTokenProviderFactory func() (AuthTokenProvider, error)

var (
	authTokenProviderFactories = make(map[string]AuthTokenProviderFactory)
	authTokenProviders         = make(map[string]AuthTokenProvider)
)

func RegisterAuthTokenProvider(name string, factory AuthTokenProviderFactory) {
	authTokenProviderFactories[name] = factory
}

func GetAuthTokenProvider(name string) (AuthTokenProvider, error) {
	if provider, ok := authTokenProviders[name]; ok {
		return provider, nil
	}

	if factory, ok := authTokenProviderFactories[name]; ok {
		provider, err := factory()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create AuthTokenProvider")
		}

		authTokenProviders[name] = provider
		return provider, nil
	} else {
		return nil, errors.Errorf("no registered AuthTokenProvider for %s", name)
	}
}
