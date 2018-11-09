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

package config_test

import (
	"github.com/racker/telemetry-envoy/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestComputeLabels(t *testing.T) {
	tests := []struct {
		name     string
		wantKeys []string
		// the key-values that definitely need to be present
		want      map[string]string
		wantErr   bool
		viperYaml string
	}{
		{name: "no config", wantKeys: []string{"os", "hostname", "arch"}},
		{name: "with labels config", wantKeys: []string{"os", "hostname", "arch", "env"},
			want:      map[string]string{"env": "prod"},
			viperYaml: "labels:\n  env: prod",
		},
		{name: "override with config", wantKeys: []string{"os", "hostname", "arch"},
			want:      map[string]string{"hostname": "hostA"},
			viperYaml: "labels:\n  hostname: hostA",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			viper.Reset()
			if tt.viperYaml != "" {
				viper.SetConfigType("yaml")
				err := viper.ReadConfig(strings.NewReader(tt.viperYaml))
				require.NoError(t, err)
			}

			got, err := config.ComputeLabels()
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil {
				for k, v := range tt.want {
					gotValue, ok := got[k]
					assert.Truef(t, ok, "missing key %s", k)
					assert.Equalf(t, v, gotValue, "wrong value for key %s", k)
				}
			}
			if tt.wantKeys != nil {
				for _, key := range tt.wantKeys {
					assert.Contains(t, got, key)
				}
			}
		})
	}
}
