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

package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"os"
	"runtime"
)

func ComputeLabels() (map[string]string, error) {
	labels := make(map[string]string)

	labels["os"] = runtime.GOOS
	labels["arch"] = runtime.GOARCH

	hostname, err := os.Hostname()
	if err == nil {
		labels["hostname"] = hostname
	} else {
		return nil, errors.Wrap(err, "unable to determine hostname label")
	}

	configuredLabels := viper.GetStringMapString("labels")
	for k, v := range configuredLabels {
		labels[k] = v
	}

	return labels, nil
}
