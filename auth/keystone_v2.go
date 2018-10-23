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
	"bytes"
	"encoding/json"
	"github.com/oliveagle/jsonpath"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"text/template"
)

type KeystoneV2AuthTokenProvider struct {
	config *KeystoneV2Config
}

type KeystoneV2Config struct {
	IdentityServiceUrl string
	Username           string
	Apikey             string
}

var tokensPostBody = `{
	"auth": {
		"RAX-KSKEY:apiKeyCredentials": {
			"username": "{{ .Username  }}",
			"apiKey": "{{ .Apikey  }}"
		}
	}
}`

func init() {
	viper.SetDefault("tls.keystone_v2.identityServiceUrl", "https://identity.api.rackspacecloud.com/v2.0")

	RegisterAuthTokenProvider("keystone_v2", func() (AuthTokenProvider, error) {
		return NewKeystoneV2AuthTokenProvider()
	})
}

func NewKeystoneV2AuthTokenProvider() (*KeystoneV2AuthTokenProvider, error) {
	var config KeystoneV2Config
	err := viper.UnmarshalKey("tls.keystone_v2", &config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	return &KeystoneV2AuthTokenProvider{
		config: &config,
	}, nil
}

func (p *KeystoneV2AuthTokenProvider) ProvideAuthToken() (*AuthToken, error) {

	if p.config.IdentityServiceUrl == "" || p.config.Username == "" || p.config.Apikey == "" {
		return nil, errors.New("identityServiceUrl, username, and apikey need to be set in tls.keystone_v2 config")
	}

	parsed, err := template.New("tokensPostBody").Parse(tokensPostBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse body template")
	}

	var postBody bytes.Buffer
	err = parsed.Execute(&postBody, &p.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build token post body")
	}

	resp, err := http.Post(p.config.IdentityServiceUrl+"/tokens", "application/json", &postBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to post request for token")
	}
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response bytes")
	}

	var respJson interface{}
	err = json.Unmarshal(respBytes, &respJson)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode response json")
	}

	resTokenId, err := jsonpath.JsonPathLookup(respJson, "$.access.token.id")
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract token from response json")
	}

	if tokenId, ok := resTokenId.(string); ok {
		return &AuthToken{
			Header: "X-Auth-Token",
			Value:  tokenId,
		}, nil
	} else {
		return nil, errors.New("failed to locate tokenId in response json")
	}
}
