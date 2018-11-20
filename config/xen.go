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
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// GetXenId will try and detect the id of any server running on the xen hypervisor.
func GetXenId() (string, error) {
	xenId, err := GetXenIdFromCloudInit()
	if err != nil {
		log.WithError(err).Debug("failed to get xen-id from cloud init")
		xenId, err = GetXenIdFromXenClient()
		if err != nil {
			log.WithError(err).Debug("failed to get xen-id from xen client")
			return "", err
		}
	}
	return xenId, err
}

// GetXenIdFromCloudInit attempts to retrieve the xen-id from the instance id file.
func GetXenIdFromCloudInit() (string, error) {
	if runtime.GOOS == "windows" {
		return "", errors.New("cloud init is not supported on windows")
	}
	instanceIdPath := "/var/lib/cloud/data/instance-id"
	data, err := ioutil.ReadFile(instanceIdPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read from instance id path")
	}
	// remove new line characters
	xenId := strings.TrimSuffix(string(data), "\n")
	xenId = strings.ToLower(xenId)
	// the fallback datasource is iid-datasource-none when it does not exist
	// https://cloudinit.readthedocs.io/en/latest/topics/datasources/fallback.html
	if xenId == "iid-datasource-none" || xenId == "nocloud" {
		return "", errors.New("invalid instance id found")
	}
	return xenId, nil
}

// GetXenIdFromXenClient attempts to retrieve the xen-id using the xenstore client.
func GetXenIdFromXenClient() (string, error) {
	var xenId string
	switch runtime.GOOS {
	case "linux":
		output, err := exec.Command("xenstore-read", "name").Output()
		if err != nil {
			return "", err
		}
		xenId = string(output)
	case "windows":
		file := "c:\\Program Files\\Citrix\\XenTools\\xenstore_client.exe"
		if _, err := os.Stat(file); os.IsNotExist(err) || err != nil {
			output, err := exec.Command("powershell",
				"& {$sid = ((Get-WmiObject -Class CitrixXenStoreBase -Namespace root\\wmi)" +
					".AddSession(\"Temp\").SessionId) ; $s = (Get-WmiObject -Namespace root\\wmi -Query " +
					"\"select * from CitrixXenStoreSession where SessionId=$sid\") ; $v = $s.GetValue(\"name\").value ;" +
					"$s.EndSession() ; $v}",
				"read", "name").Output()
			if err != nil {
				return "", err
			}
			xenId = string(output)
		} else {
			output, err := exec.Command(file, "name").Output()
			if err != nil {
				return "", err
			}
			xenId = string(output)
		}
	default:
		return "", errors.New("no xen id found on os with type " + runtime.GOOS)
	}

	// remove new line characters
	xenId = strings.TrimSuffix(strings.TrimSuffix(xenId, "\n"), "\r")
	xenId = strings.ToLower(xenId)

	return xenId, nil
}