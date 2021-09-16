/*
Copyright 2019 The logr Authors.

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

// Copyright 2018 Solly Ross
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package zapr defines an implementation of the github.com/go-logr/logr
// interfaces built on top of Zap (go.uber.org/zap).
//
// Usage
//
// A new logr.Logger can be constructed from an existing zap.Logger using
// the NewLogger function:
//
//  log := zapr.NewLogger(someZapLogger)
//
// Implementation Details
//
// For the most part, concepts in Zap correspond directly with those in
// logr.
//
// Unlike Zap, all fields *must* be in the form of sugared fields --
// it's illegal to pass a strongly-typed Zap field in a key position
// to any of the log methods.
//
// Levels in logr correspond to custom debug levels in Zap.  Any given level
// in logr is represents by its inverse in zap (`zapLevel = -1*logrLevel`).
// For example V(2) is equivalent to log level -2 in Zap, while V(1) is
// equivalent to Zap's DebugLevel.
package zapr

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NB: right now, we always use the equivalent of sugared logging.
// This is necessary, since logr doesn't define non-suggared types,
// and using zap-specific non-suggared types would make uses tied
// directly to Zap.

// zapLogger is a logr.Logger that uses Zap to log.  The level has already been
// converted to a Zap level, which is to say that `logrLevel = -1*zapLevel`.
type zapLogger struct {
	// NB: this looks very similar to zap.SugaredLogger, but
	// deals with our desire to have multiple verbosity levels.
	l *zap.Logger

	// numericLevelKey controls whether the numeric logr level is
	// added to each Info log message and with which key.
	numericLevelKey string

	// errorKey is the field name used for the error in
	// Logger.Error calls.
	errorKey string

	// allowZapFields enables logging of strongly-typed Zap
	// fields. It is off by default because it breaks
	// implementation agnosticism.
	allowZapFields bool

	// panicMessages enables log messages for invalid log calls
	// that explain why a call was invalid (for example,
	// non-string key). This is enabled by default.
	panicMessages bool
}

const (
	// noLevel tells handleFields to not inject a numeric log level field.
	noLevel = -1
)

// handleFields converts a bunch of arbitrary key-value pairs into Zap fields.  It takes
// additional pre-converted Zap fields, for use with automatically attached fields, like
// `error`.
func (zl *zapLogger) handleFields(lvl int, args []interface{}, additional ...zap.Field) []zap.Field {
	injectNumericLevel := zl.numericLevelKey != "" && lvl != noLevel

	// a slightly modified version of zap.SugaredLogger.sweetenFields
	if len(args) == 0 {
		// fast-return if we have no suggared fields and no "v" field.
		if !injectNumericLevel {
			return additional
		}
		// Slightly slower fast path when we need to inject "v".
		return append(additional, zap.Int(zl.numericLevelKey, lvl))
	}

	// unlike Zap, we can be pretty sure users aren't passing structured
	// fields (since logr has no concept of that), so guess that we need a
	// little less space.
	numFields := len(args)/2 + len(additional)
	if injectNumericLevel {
		numFields++
	}
	fields := make([]zap.Field, 0, numFields)
	if injectNumericLevel {
		fields = append(fields, zap.Int(zl.numericLevelKey, lvl))
	}
	for i := 0; i < len(args); {
		// Check just in case for strongly-typed Zap fields,
		// which might be illegal (since it breaks
		// implementation agnosticism). If disabled, we can
		// give a better error message.
		if field, ok := args[i].(zap.Field); ok {
			if zl.allowZapFields {
				fields = append(fields, field)
				i++
				continue
			}
			if zl.panicMessages {
				zl.l.WithOptions(zap.AddCallerSkip(1)).DPanic("strongly-typed Zap Field passed to logr", zapAny("zap field", args[i]))
			}
			break
		}

		// make sure this isn't a mismatched key
		if i == len(args)-1 {
			if zl.panicMessages {
				zl.l.WithOptions(zap.AddCallerSkip(1)).DPanic("odd number of arguments passed as key-value pairs for logging", zapAny("ignored key", args[i]))
			}
			break
		}

		// process a key-value pair,
		// ensuring that the key is a string
		key, val := args[i], args[i+1]
		keyStr, isString := key.(string)
		if !isString {
			// if the key isn't a string, DPanic and stop logging
			if zl.panicMessages {
				zl.l.WithOptions(zap.AddCallerSkip(1)).DPanic("non-string key argument passed to logging, ignoring all later arguments", zapAny("invalid key", key))
			}
			break
		}

		fields = append(fields, zapAny(keyStr, val))
		i += 2
	}

	return append(fields, additional...)
}

// TODO: move to logr
//
// UseReflection is an optional interface that values in a key/value list
// may implement. That this interface is implemented provides a hint to
// loggers that all fields in the value are meant to be logged via
// reflection even when the logger normally would do something else,
// like using the Stringer interface.
//
// Loggers are not required to support it. Depending on their purpose
// or configuration, they might still prefer to use the Stringer interface,
// for example when the goal is to provide human-readable output.
type UseReflection interface {
	// LogWithReflection will never get called and can be empty.
	// It's existence already indicates the intent to prefer reflection
	// for logging over alternatives like String.
	LogWithReflection()
}

// zapAny is a fork of https://github.com/uber-go/zap/blob/eaeb0fc72fd23af7969c9a9f39e51b66827507ca/field.go#L413-L549
// which adds support for logr.UseReflection
func zapAny(key string, value interface{}) zapcore.Field {
	switch val := value.(type) {
	case UseReflection:
		return zap.Reflect(key, val)
	case zapcore.ObjectMarshaler:
		return zap.Object(key, val)
	case zapcore.ArrayMarshaler:
		return zap.Array(key, val)
	case bool:
		return zap.Bool(key, val)
	case *bool:
		return zap.Boolp(key, val)
	case []bool:
		return zap.Bools(key, val)
	case complex128:
		return zap.Complex128(key, val)
	case *complex128:
		return zap.Complex128p(key, val)
	case []complex128:
		return zap.Complex128s(key, val)
	case complex64:
		return zap.Complex64(key, val)
	case *complex64:
		return zap.Complex64p(key, val)
	case []complex64:
		return zap.Complex64s(key, val)
	case float64:
		return zap.Float64(key, val)
	case *float64:
		return zap.Float64p(key, val)
	case []float64:
		return zap.Float64s(key, val)
	case float32:
		return zap.Float32(key, val)
	case *float32:
		return zap.Float32p(key, val)
	case []float32:
		return zap.Float32s(key, val)
	case int:
		return zap.Int(key, val)
	case *int:
		return zap.Intp(key, val)
	case []int:
		return zap.Ints(key, val)
	case int64:
		return zap.Int64(key, val)
	case *int64:
		return zap.Int64p(key, val)
	case []int64:
		return zap.Int64s(key, val)
	case int32:
		return zap.Int32(key, val)
	case *int32:
		return zap.Int32p(key, val)
	case []int32:
		return zap.Int32s(key, val)
	case int16:
		return zap.Int16(key, val)
	case *int16:
		return zap.Int16p(key, val)
	case []int16:
		return zap.Int16s(key, val)
	case int8:
		return zap.Int8(key, val)
	case *int8:
		return zap.Int8p(key, val)
	case []int8:
		return zap.Int8s(key, val)
	case string:
		return zap.String(key, val)
	case *string:
		return zap.Stringp(key, val)
	case []string:
		return zap.Strings(key, val)
	case uint:
		return zap.Uint(key, val)
	case *uint:
		return zap.Uintp(key, val)
	case []uint:
		return zap.Uints(key, val)
	case uint64:
		return zap.Uint64(key, val)
	case *uint64:
		return zap.Uint64p(key, val)
	case []uint64:
		return zap.Uint64s(key, val)
	case uint32:
		return zap.Uint32(key, val)
	case *uint32:
		return zap.Uint32p(key, val)
	case []uint32:
		return zap.Uint32s(key, val)
	case uint16:
		return zap.Uint16(key, val)
	case *uint16:
		return zap.Uint16p(key, val)
	case []uint16:
		return zap.Uint16s(key, val)
	case uint8:
		return zap.Uint8(key, val)
	case *uint8:
		return zap.Uint8p(key, val)
	case []byte:
		return zap.Binary(key, val)
	case uintptr:
		return zap.Uintptr(key, val)
	case *uintptr:
		return zap.Uintptrp(key, val)
	case []uintptr:
		return zap.Uintptrs(key, val)
	case time.Time:
		return zap.Time(key, val)
	case *time.Time:
		return zap.Timep(key, val)
	case []time.Time:
		return zap.Times(key, val)
	case time.Duration:
		return zap.Duration(key, val)
	case *time.Duration:
		return zap.Durationp(key, val)
	case []time.Duration:
		return zap.Durations(key, val)
	case error:
		return zap.NamedError(key, val)
	case []error:
		return zap.Errors(key, val)
	case fmt.Stringer:
		return zap.Stringer(key, val)
	default:
		return zap.Reflect(key, val)
	}
}

func zapAny2(key string, value interface{}) zapcore.Field {
	if val, ok := value.(UseReflection); ok {
		return zap.Reflect(key, val)
	}
	return zap.Any(key, value)
}

func zapAny3(key string, value interface{}) zapcore.Field {
	if val, ok := value.(UseReflection); ok {
		return zap.Reflect(key, val)
	}
	if val, ok := value.(zapcore.ObjectMarshaler); ok {
		return zap.Object(key, val)
	}
	if val, ok := value.(zapcore.ArrayMarshaler); ok {
		return zap.Array(key, val)
	}
	if val, ok := value.(bool); ok {
		return zap.Bool(key, val)
	}
	if val, ok := value.(*bool); ok {
		return zap.Boolp(key, val)
	}
	if val, ok := value.([]bool); ok {
		return zap.Bools(key, val)
	}
	if val, ok := value.(complex128); ok {
		return zap.Complex128(key, val)
	}
	if val, ok := value.(*complex128); ok {
		return zap.Complex128p(key, val)
	}
	if val, ok := value.([]complex128); ok {
		return zap.Complex128s(key, val)
	}
	if val, ok := value.(complex64); ok {
		return zap.Complex64(key, val)
	}
	if val, ok := value.(*complex64); ok {
		return zap.Complex64p(key, val)
	}
	if val, ok := value.([]complex64); ok {
		return zap.Complex64s(key, val)
	}
	if val, ok := value.(float64); ok {
		return zap.Float64(key, val)
	}
	if val, ok := value.(*float64); ok {
		return zap.Float64p(key, val)
	}
	if val, ok := value.([]float64); ok {
		return zap.Float64s(key, val)
	}
	if val, ok := value.(float32); ok {
		return zap.Float32(key, val)
	}
	if val, ok := value.(*float32); ok {
		return zap.Float32p(key, val)
	}
	if val, ok := value.([]float32); ok {
		return zap.Float32s(key, val)
	}
	if val, ok := value.(int); ok {
		return zap.Int(key, val)
	}
	if val, ok := value.(*int); ok {
		return zap.Intp(key, val)
	}
	if val, ok := value.([]int); ok {
		return zap.Ints(key, val)
	}
	if val, ok := value.(int64); ok {
		return zap.Int64(key, val)
	}
	if val, ok := value.(*int64); ok {
		return zap.Int64p(key, val)
	}
	if val, ok := value.([]int64); ok {
		return zap.Int64s(key, val)
	}
	if val, ok := value.(int32); ok {
		return zap.Int32(key, val)
	}
	if val, ok := value.(*int32); ok {
		return zap.Int32p(key, val)
	}
	if val, ok := value.([]int32); ok {
		return zap.Int32s(key, val)
	}
	if val, ok := value.(int16); ok {
		return zap.Int16(key, val)
	}
	if val, ok := value.(*int16); ok {
		return zap.Int16p(key, val)
	}
	if val, ok := value.([]int16); ok {
		return zap.Int16s(key, val)
	}
	if val, ok := value.(int8); ok {
		return zap.Int8(key, val)
	}
	if val, ok := value.(*int8); ok {
		return zap.Int8p(key, val)
	}
	if val, ok := value.([]int8); ok {
		return zap.Int8s(key, val)
	}
	if val, ok := value.(string); ok {
		return zap.String(key, val)
	}
	if val, ok := value.(*string); ok {
		return zap.Stringp(key, val)
	}
	if val, ok := value.([]string); ok {
		return zap.Strings(key, val)
	}
	if val, ok := value.(uint); ok {
		return zap.Uint(key, val)
	}
	if val, ok := value.(*uint); ok {
		return zap.Uintp(key, val)
	}
	if val, ok := value.([]uint); ok {
		return zap.Uints(key, val)
	}
	if val, ok := value.(uint64); ok {
		return zap.Uint64(key, val)
	}
	if val, ok := value.(*uint64); ok {
		return zap.Uint64p(key, val)
	}
	if val, ok := value.([]uint64); ok {
		return zap.Uint64s(key, val)
	}
	if val, ok := value.(uint32); ok {
		return zap.Uint32(key, val)
	}
	if val, ok := value.(*uint32); ok {
		return zap.Uint32p(key, val)
	}
	if val, ok := value.([]uint32); ok {
		return zap.Uint32s(key, val)
	}
	if val, ok := value.(uint16); ok {
		return zap.Uint16(key, val)
	}
	if val, ok := value.(*uint16); ok {
		return zap.Uint16p(key, val)
	}
	if val, ok := value.([]uint16); ok {
		return zap.Uint16s(key, val)
	}
	if val, ok := value.(uint8); ok {
		return zap.Uint8(key, val)
	}
	if val, ok := value.(*uint8); ok {
		return zap.Uint8p(key, val)
	}
	if val, ok := value.([]byte); ok {
		return zap.Binary(key, val)
	}
	if val, ok := value.(uintptr); ok {
		return zap.Uintptr(key, val)
	}
	if val, ok := value.(*uintptr); ok {
		return zap.Uintptrp(key, val)
	}
	if val, ok := value.([]uintptr); ok {
		return zap.Uintptrs(key, val)
	}
	if val, ok := value.(time.Time); ok {
		return zap.Time(key, val)
	}
	if val, ok := value.(*time.Time); ok {
		return zap.Timep(key, val)
	}
	if val, ok := value.([]time.Time); ok {
		return zap.Times(key, val)
	}
	if val, ok := value.(time.Duration); ok {
		return zap.Duration(key, val)
	}
	if val, ok := value.(*time.Duration); ok {
		return zap.Durationp(key, val)
	}
	if val, ok := value.([]time.Duration); ok {
		return zap.Durations(key, val)
	}
	if val, ok := value.(error); ok {
		return zap.NamedError(key, val)
	}
	if val, ok := value.([]error); ok {
		return zap.Errors(key, val)
	}
	if val, ok := value.(fmt.Stringer); ok {
		return zap.Stringer(key, val)
	}

	return zap.Reflect(key, value)
}

func (zl *zapLogger) Init(ri logr.RuntimeInfo) {
	zl.l = zl.l.WithOptions(zap.AddCallerSkip(ri.CallDepth))
}

// Zap levels are int8 - make sure we stay in bounds.  logr itself should
// ensure we never get negative values.
func toZapLevel(lvl int) zapcore.Level {
	if lvl > 127 {
		lvl = 127
	}
	// zap levels are inverted.
	return 0 - zapcore.Level(lvl)
}

func (zl zapLogger) Enabled(lvl int) bool {
	return zl.l.Core().Enabled(toZapLevel(lvl))
}

func (zl *zapLogger) Info(lvl int, msg string, keysAndVals ...interface{}) {
	if checkedEntry := zl.l.Check(toZapLevel(lvl), msg); checkedEntry != nil {
		checkedEntry.Write(zl.handleFields(lvl, keysAndVals)...)
	}
}

func (zl *zapLogger) Error(err error, msg string, keysAndVals ...interface{}) {
	if checkedEntry := zl.l.Check(zap.ErrorLevel, msg); checkedEntry != nil {
		checkedEntry.Write(zl.handleFields(noLevel, keysAndVals, zap.NamedError(zl.errorKey, err))...)
	}
}

func (zl *zapLogger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	newLogger := *zl
	newLogger.l = zl.l.With(zl.handleFields(noLevel, keysAndValues)...)
	return &newLogger
}

func (zl *zapLogger) WithName(name string) logr.LogSink {
	newLogger := *zl
	newLogger.l = zl.l.Named(name)
	return &newLogger
}

func (zl *zapLogger) WithCallDepth(depth int) logr.LogSink {
	newLogger := *zl
	newLogger.l = zl.l.WithOptions(zap.AddCallerSkip(depth))
	return &newLogger
}

// Underlier exposes access to the underlying logging implementation.  Since
// callers only have a logr.Logger, they have to know which implementation is
// in use, so this interface is less of an abstraction and more of way to test
// type conversion.
type Underlier interface {
	GetUnderlying() *zap.Logger
}

func (zl *zapLogger) GetUnderlying() *zap.Logger {
	return zl.l
}

// NewLogger creates a new logr.Logger using the given Zap Logger to log.
func NewLogger(l *zap.Logger) logr.Logger {
	return NewLoggerWithOptions(l)
}

// NewLoggerWithOptions creates a new logr.Logger using the given Zap Logger to
// log and applies additional options.
func NewLoggerWithOptions(l *zap.Logger, opts ...Option) logr.Logger {
	// creates a new logger skipping one level of callstack
	log := l.WithOptions(zap.AddCallerSkip(1))
	zl := &zapLogger{
		l: log,
	}
	zl.errorKey = "error"
	zl.panicMessages = true
	for _, option := range opts {
		option(zl)
	}
	return logr.New(zl)
}

// Option is one additional parameter for NewLoggerWithOptions.
type Option func(*zapLogger)

// LogInfoLevel controls whether a numeric log level is added to
// Info log message. The empty string disables this, a non-empty
// string is the key for the additional field. Errors and
// internal panic messages do not have a log level and thus
// are always logged without this extra field.
func LogInfoLevel(key string) Option {
	return func(zl *zapLogger) {
		zl.numericLevelKey = key
	}
}

// ErrorKey replaces the default "error" field name used for the error
// in Logger.Error calls.
func ErrorKey(key string) Option {
	return func(zl *zapLogger) {
		zl.errorKey = key
	}
}

// AllowZapFields controls whether strongly-typed Zap fields may
// be passed instead of a key/value pair. This is disabled by
// default because it breaks implementation agnosticism.
func AllowZapFields(allowed bool) Option {
	return func(zl *zapLogger) {
		zl.allowZapFields = allowed
	}
}

// DPanicOnBugs controls whether extra log messages are emitted for
// invalid log calls with zap's DPanic method. Depending on the
// configuration of the zap logger, the program then panics after
// emitting the log message which is useful in development because
// such invalid log calls are bugs in the program. The log messages
// explain why a call was invalid (for example, non-string
// key). Emitting them is enabled by default.
func DPanicOnBugs(enabled bool) Option {
	return func(zl *zapLogger) {
		zl.panicMessages = enabled
	}
}

var _ logr.LogSink = &zapLogger{}
var _ logr.CallDepthLogSink = &zapLogger{}
