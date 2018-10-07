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

//go:generate pegomock generate --package=agents agents.go

package agents_test

import (
	"context"
	"github.com/petergtz/pegomock"
	"reflect"
)

func AnyContext() context.Context {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*context.Context)(nil)).Elem()))
	return context.Background()
}
