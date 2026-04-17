// Copyright 2026 ptrvsrg.
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

package framework

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	runIDLabelKey      = "e2e.ptrvsrg.io/run-id"
	testSuiteLabelKey  = "e2e.ptrvsrg.io/suite"
	testScenarioLabel  = "e2e.ptrvsrg.io/scenario"
	cleanupGracePeriod = 15 * time.Second
)

// Framework is a shared utility holder for a single suite.
type Framework struct {
	Config Config

	Kube      kubernetes.Interface
	Dynamic   dynamic.Interface
	Snapshots snapshotv1.Interface
	RestCfg   *rest.Config

	RunID    string
	SuiteTag string

	logger   *slog.Logger
	rnd      *rand.Rand
	mu       sync.Mutex
	cleanups []func(context.Context) error
}

// New initializes framework clients and suite run identity.
func New(suiteTag string) (*Framework, error) {
	cfg := LoadConfig()
	core, dyn, snap, restCfg, err := NewClients()
	if err != nil {
		return nil, err
	}

	rnd := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano())))
	runID := fmt.Sprintf("%s-%d-%04d", sanitizeForLabel(suiteTag), time.Now().Unix(), rnd.IntN(10000))

	return &Framework{
		Config:    cfg,
		Kube:      core,
		Dynamic:   dyn,
		Snapshots: snap,
		RestCfg:   restCfg,
		RunID:     runID,
		SuiteTag:  sanitizeForLabel(suiteTag),
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
		rnd: rnd,
	}, nil
}

// MustRun returns whether suite execution is enabled by environment.
func (f *Framework) MustRun() bool {
	return f.Config.Run
}

// Labels returns baseline labels for all created test resources.
func (f *Framework) Labels(extra map[string]string) map[string]string {
	out := map[string]string{
		runIDLabelKey:     f.RunID,
		testSuiteLabelKey: f.SuiteTag,
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// RegisterCleanup adds cleanup callback executed in LIFO order at suite teardown.
func (f *Framework) RegisterCleanup(fn func(context.Context) error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups = append(f.cleanups, fn)
	f.logOperation("cleanup register", "count", len(f.cleanups))
}

// Cleanup executes registered cleanup handlers in reverse order.
func (f *Framework) Cleanup(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var allErr error
	f.logOperation("cleanup start", "registered", len(f.cleanups))
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		cleanupCtx, cancel := context.WithTimeout(ctx, cleanupGracePeriod)
		err := f.cleanups[i](cleanupCtx)
		cancel()
		if err != nil {
			f.logOperation("cleanup step failed", "index", i, "error", err)
			allErr = errors.Join(allErr, err)
			continue
		}
		f.logOperation("cleanup step done", "index", i)
	}
	f.cleanups = nil
	if allErr != nil {
		f.logOperation("cleanup finished", "result", "failed", "error", allErr)
		return allErr
	}
	f.logOperation("cleanup finished", "result", "ok")
	return allErr
}

func sanitizeForLabel(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	in = strings.ReplaceAll(in, "_", "-")
	in = strings.ReplaceAll(in, "/", "-")
	in = strings.ReplaceAll(in, " ", "-")
	if in == "" {
		return "suite"
	}
	return in
}

// NewName creates unique deterministic names per suite run.
func (f *Framework) NewName(prefix string) string {
	name := fmt.Sprintf("%s-%s-%04d", prefix, f.RunID, f.rnd.IntN(10000))
	f.logOperation("name new", "prefix", prefix, "value", name)
	return name
}

// SelectorForRun returns label selector for run cleanup and debugging.
func (f *Framework) SelectorForRun() string {
	return metav1.FormatLabelSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			runIDLabelKey: f.RunID,
		},
	})
}

func (f *Framework) logOperation(msg string, kv ...any) {
	attrs := []any{
		"suite", f.SuiteTag,
		"run_id", f.RunID,
	}
	attrs = append(attrs, kv...)
	f.logger.Info(msg, attrs...)
}

func (f *Framework) withRetries(ctx context.Context, action string, fn func(context.Context) error) error {
	attempts := f.Config.RetryAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(context.Background(), f.Config.Timeout)
		err := fn(attemptCtx)
		cancel()
		if err == nil {
			if attempt > 1 {
				f.logOperation("retry succeeded", "action", action, "attempt", attempt)
			}
			return nil
		}

		lastErr = err
		f.logOperation("retry failed", "action", action, "attempt", attempt, "attempts", attempts, "error", err)
		_ = f.DumpDiagnostics(ctx, f.Config.Namespace)
		if attempt < attempts {
			time.Sleep(f.Config.RetryDelay)
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", action, attempts, lastErr)
}
