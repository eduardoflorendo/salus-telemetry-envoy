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

//go:generate pegomock generate github.com/racker/telemetry-envoy/ambassador EgressConnection

package ingest_test

import (
	"github.com/petergtz/pegomock"
	"github.com/racker/telemetry-envoy/telemetry_edge"
	"reflect"
)

func AnyMetric() *telemetry_edge.Metric {
	var m *telemetry_edge.Metric
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf(m)))
	return m
}
