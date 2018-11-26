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
	"os/exec"
	"runtime"
	"strings"
)

// GetBiosData returns the bios vendor and version of the running system
func GetBiosData() (map[string]string, error) {
	biosLabels := make(map[string]string)
	switch runtime.GOOS {
	case "linux":
		data, err := GetLinuxBiosData()
		if err != nil {
			log.WithError(err).Debug("failed to get linux bios data")
			return nil, err
		}
		biosLabels = data
	case "windows":
		data, err := GetWindowsBiosData()
		if err != nil {
			log.WithError(err).Debug("failed to get windows bios data")
			return nil, err
		}
		biosLabels = data
	default:
		return nil, errors.New("no bios data found on os with type " + runtime.GOOS)
	}

	return biosLabels, nil
}

// GetWindowsBiosData is a helper function to get the bios vendor and version for windows systems
func GetWindowsBiosData() (map[string]string, error) {
	var vendor, version string

	output, err := exec.Command("powershell", "(Get-WmiObject win32_bios).Manufacturer").Output()
	if err != nil {
		log.WithError(err).Debug("failed to execute powershell command to gather bios manufacturer")
		return nil, err
	}
	vendor = strings.TrimSpace(string(output))
	vendor = strings.ToLower(vendor)

	output, err = exec.Command("powershell", "(Get-WmiObject win32_bios).SMBIOSBIOSVersion").Output()
	if err != nil {
		log.WithError(err).Debug("failed to execute powershell command to gather bios version")
		return nil, err
	}
	version = strings.TrimSpace(string(output))
	version = strings.ToLower(version)

	biosLabels := map[string]string{
		BiosVendorLabel: vendor,
		BiosVersionLabel: version,
	}

	return biosLabels, nil
}

// GetLinuxBiosData is a helper function to get the bios vendor and version for linux systems
func GetLinuxBiosData() (map[string]string, error) {
	var vendor, version string

	output, err := exec.Command("dmidecode", "-s", "bios-vendor").Output()
	if err != nil {
		return nil, err
	}
	vendor = strings.TrimSpace(string(output))
	vendor = strings.ToLower(vendor)

	output, err = exec.Command("dmidecode", "-s", "bios-version").Output()
	if err != nil {
		return nil, err
	}
	version = strings.TrimSpace(string(output))
	version = strings.ToLower(version)

	biosLabels := map[string]string{
		BiosVendorLabel: vendor,
		BiosVersionLabel: version,
	}

	return biosLabels, nil
}