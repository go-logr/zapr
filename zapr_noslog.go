//go:build !go1.21

/*
Copyright 2023 The logr Authors.

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
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (zl *zapLogger) zapIt(field string, val interface{}) zap.Field {
	// Handle types that implement logr.Marshaler: log the replacement
	// object instead of the original one.
	if marshaler, ok := val.(logr.Marshaler); ok {
		field, val = invokeMarshaler(field, marshaler)
	}
	return zap.Any(field, val)
}

type errorDetailer interface {
	ErrorDetails() any
}

func (zl *zapLogger) zapError(field string, err error) zap.Field {
	if err == nil {
		return zap.Skip()
	}
	return zap.Inline(zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
		// Always log as a normal error first.
		zap.NamedError(field, err).AddTo(encoder)

		// Extra details are optional, but might be available if the error also
		// implements slog.LogValuer.
		if v, ok := err.(errorDetailer); ok {
			field := field + zl.errorKeyDetailsSuffix
			field, value := invokeErrorDetailer(field, v.ErrorDetails)
			zl.zapIt(field, value).AddTo(encoder)
		}
		return nil
	}))
}
