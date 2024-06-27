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

package zapr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var unsupportedTests = func() map[string]bool {
	m := make(map[string]bool)
	for _, name := range []string{
		// Cannot avoid empty groups.
		"empty-group-record",
		"nested-empty-group-record",
	} {
		m[name] = true
	}
	return m
}()

func TestSlogHandler(t *testing.T) {
	var buffer bytes.Buffer
	slogtest.Run(t, func(t *testing.T) slog.Handler {
		name := t.Name()
		name = strings.SplitN(name, "/", 2)[1]
		if unsupportedTests[name] {
			t.Skip()
		}

		encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			MessageKey: slog.MessageKey,
			TimeKey:    slog.TimeKey,
			LevelKey:   slog.LevelKey,
			EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
				encoder.AppendInt(int(level))
			},
		})

		buffer.Reset()
		core := zapcore.NewCore(encoder, zapcore.AddSync(&buffer), zapcore.Level(0))
		zl := zap.New(core)
		logger := zapr.NewLogger(zl)
		handler := logr.ToSlogHandler(logger)

		return handler
	}, func(*testing.T) map[string]any {
		return parseOutput(t, buffer.Bytes())
	})
}

// TestSlogCases covers some gaps in the coverage we get from
// slogtest.TestHandler (empty and invalud PC, see
// https://github.com/golang/go/issues/62280) and verbosity handling in
// combination with V().
func TestSlogCases(t *testing.T) {
	for name, tc := range map[string]struct {
		record   slog.Record
		v        int
		expected string
	}{
		"empty": {
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"invalid-pc": {
			record:   slog.Record{PC: 1},
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"debug": {
			record:   slog.Record{Level: slog.LevelDebug},
			expected: `{"msg":"", "level":"Level(-4)", "v":4}`,
		},
		"warn": {
			record:   slog.Record{Level: slog.LevelWarn},
			expected: `{"msg":"", "level":"warn", "v":0}`,
		},
		"error": {
			record:   slog.Record{Level: slog.LevelError},
			expected: `{"msg":"", "level":"error"}`,
		},
		"debug-v1": {
			v:        1,
			record:   slog.Record{Level: slog.LevelDebug},
			expected: `{"msg":"", "level":"Level(-5)", "v":5}`,
		},
		"warn-v1": {
			v:        1,
			record:   slog.Record{Level: slog.LevelWarn},
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"error-v1": {
			v:        1,
			record:   slog.Record{Level: slog.LevelError},
			expected: `{"msg":"", "level":"error"}`,
		},
		"debug-v4": {
			v:        4,
			record:   slog.Record{Level: slog.LevelDebug},
			expected: `{"msg":"", "level":"Level(-8)", "v":8}`,
		},
		"warn-v4": {
			v:        4,
			record:   slog.Record{Level: slog.LevelWarn},
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"error-v4": {
			v:        4,
			record:   slog.Record{Level: slog.LevelError},
			expected: `{"msg":"", "level":"error"}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer
			encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				MessageKey: slog.MessageKey,
				LevelKey:   slog.LevelKey,
				EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
					encoder.AppendString(level.String())
				},
			})
			core := zapcore.NewCore(encoder, zapcore.AddSync(&buffer), zapcore.Level(-10))
			zl := zap.New(core)
			logger := zapr.NewLoggerWithOptions(zl, zapr.LogInfoLevel("v"))
			handler := logr.ToSlogHandler(logger.V(tc.v))
			require.NoError(t, handler.Handle(context.Background(), tc.record))
			_ = zl.Sync()
			require.JSONEq(t, tc.expected, buffer.String())
		})
	}
}

func parseOutput(t *testing.T, output []byte) map[string]any {
	var m map[string]any
	if err := json.Unmarshal(output, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
