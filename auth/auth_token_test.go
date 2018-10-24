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

package auth_test

import (
	"github.com/pkg/errors"
	"github.com/racker/telemetry-envoy/auth"
	"github.com/stretchr/testify/assert"
	"testing"
)

type TestAuthTokenProvider struct {
	Header string
	Token  string
}

func (p *TestAuthTokenProvider) ProvideAuthToken() (*auth.AuthToken, error) {
	return &auth.AuthToken{Header: p.Header, Value: p.Token}, nil
}

func TestGetAuthTokenProvider_Normal(t *testing.T) {

	var provider TestAuthTokenProvider

	auth.RegisterAuthTokenProvider("TestGetAuthTokenProvider_Normal", func() (auth.AuthTokenProvider, error) {
		return &provider, nil
	})

	result, err := auth.GetAuthTokenProvider("TestGetAuthTokenProvider_Normal")
	assert.NoError(t, err)
	assert.Equal(t, &provider, result)
}

func TestGetAuthTokenProvider_ErrorInFactory(t *testing.T) {

	auth.RegisterAuthTokenProvider("TestGetAuthTokenProvider_ErrorInFactory", func() (auth.AuthTokenProvider, error) {
		return nil, errors.New("something went wrong")
	})

	result, err := auth.GetAuthTokenProvider("TestGetAuthTokenProvider_ErrorInFactory")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetAuthTokenProvider_InvalidName(t *testing.T) {
	result, err := auth.GetAuthTokenProvider("foo")
	assert.Error(t, err)
	assert.Nil(t, result)
}
