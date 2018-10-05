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

package ingest

import (
	"context"
	"github.com/racker/telemetry-envoy/ambassador"
)

type Ingestor interface {
	Bind(conn ambassador.EgressConnection) error
	Start(ctx context.Context)
}

var ingestors []Ingestor

func registerIngestor(ingestor Ingestor) {
	ingestors = append(ingestors, ingestor)
}

func Ingestors() []Ingestor {
	if ingestors == nil {
		return []Ingestor{}
	} else {
		return ingestors
	}
}
