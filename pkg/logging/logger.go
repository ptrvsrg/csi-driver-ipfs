// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package logging configures the global zerolog logger and optional hooks (PID, goroutine id).
package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/timandy/routine"
)

const (
	LogEncodingJSON = "json"
	LogEncodingText = "text"

	LogLevelTrace = "trace"
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// SetupLogger configures the process-wide zerolog.Logger on package log.
//
// level selects the global minimum level (trace/debug/info/warn/error; unknown defaults to info).
// encoding must be LogEncodingJSON or LogEncodingText (console writer with timestamps).
func SetupLogger(level, encoding string) {
	var writer io.Writer
	if encoding == LogEncodingJSON {
		writer = os.Stdout
	} else {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}
	zerolog.SetGlobalLevel(parseLevel(level))
	log.Logger = zerolog.
		New(writer).
		Hook(PIDHook{}, GIDHook{}).
		With().
		Timestamp().
		Caller().
		Logger()
}

func parseLevel(s string) zerolog.Level {
	switch s {
	case LogLevelTrace:
		return zerolog.TraceLevel
	case LogLevelDebug:
		return zerolog.DebugLevel
	case LogLevelInfo:
		return zerolog.InfoLevel
	case LogLevelWarn:
		return zerolog.WarnLevel
	case LogLevelError:
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// PIDHook injects the current process id into every log event.
type PIDHook struct{}

// Run implements zerolog.Hook.
func (h PIDHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	e.Int("pid", os.Getpid())
}

// GIDHook injects the current goroutine id (via routine.Goid) into every log event.
type GIDHook struct{}

// Run implements zerolog.Hook.
func (h GIDHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	e.Uint64("gid", routine.Goid())
}
