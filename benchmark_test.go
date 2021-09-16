/*
Copyright 2021 The logr Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package zapr

import (
	"testing"

	"go.uber.org/zap"
)

var field zap.Field

//go:noinline
func doZapAny(b *testing.B, value interface{}) {
	var f zap.Field
	for i := 0; i < b.N; i++ {
		f = zapAny("value", value)
	}
	field = f
}

// Different types have different overhead, depending on how soon a casse
// matches in
// https://github.com/uber-go/zap/blob/eaeb0fc72fd23af7969c9a9f39e51b66827507ca/field.go#L413-L549

func BenchmarkZapAny_kmeta(b *testing.B) {
	doZapAny(b, KMeta{Namespace: "foo", Name: "bar"})
}

func BenchmarkZapAny_kmeta2(b *testing.B) {
	doZapAny(b, KMeta2{KMeta: KMeta{Namespace: "foo", Name: "bar"}})
}

func BenchmarkZapAny_int(b *testing.B) {
	doZapAny(b, 42)
}

func BenchmarkZapAny_bool(b *testing.B) {
	doZapAny(b, true)
}
