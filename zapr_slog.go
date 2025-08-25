//go:build go1.21
// +build go1.21

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
	"log/slog"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (zl *zapLogger) zapIt(field string, val interface{}) zap.Field {
	switch valTyped := val.(type) {
	case error:
		return zl.zapError(field, valTyped)
	case logr.Marshaler:
		// Handle types that implement logr.Marshaler: log the replacement
		// object instead of the original one.
		field, val = invokeMarshaler(field, valTyped)
	case slog.LogValuer:
		// The same for slog.LogValuer. We let slog.Value handle
		// potential panics and recursion.
		val = slog.AnyValue(val).Resolve()
	}
	if slogValue, ok := val.(slog.Value); ok {
		return zl.zapInline(field, slogValue)
	}
	return zap.Any(field, val)
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
		if valuer, ok := err.(slog.LogValuer); ok {
			value := slog.AnyValue(valuer)
			value = value.Resolve()
			zl.zapInline(field+zl.errorKeyDetailsSuffix, value).AddTo(encoder)
		}
		return nil
	}))
}

func (zl *zapLogger) zapInline(key string, value slog.Value) zap.Field {
	return zap.Inline(zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
		zl.encodeSlog(enc, slog.Attr{Key: key, Value: value})
		return nil
	}))
}
