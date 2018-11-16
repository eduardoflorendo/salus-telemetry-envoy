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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"runtime"
)

// ComputeLabels reads any labels specified in the config file.
// It also auto discovers various instance identifiers such as the os type and hostname.
// If an auto-detected label is also specified in the config file, the config file value will be used.
// These labels are passed over to the server-side endpoint.
func ComputeLabels() (map[string]string, error) {
	labels := make(map[string]string)

	labels["os"] = runtime.GOOS
	labels["arch"] = runtime.GOARCH

	hostname, err := os.Hostname()
	if err != nil {
		return nil, errors.Wrap(err, "unable to determine hostname label")
	}
	labels["hostname"] = hostname

	xenId, err := GetXenId()
	if err != nil {
		log.WithError(err).Debug("unable to determine xen-id")
	} else {
		labels["xen-id"] = xenId
	}

	configuredLabels := viper.GetStringMapString("labels")
	for k, v := range configuredLabels {
		labels[k] = v
	}

	log.WithField("labels", labels).Debug("discovered labels")

	return labels, nil
}