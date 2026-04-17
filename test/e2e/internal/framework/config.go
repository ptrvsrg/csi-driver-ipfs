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
	"os"
	"strconv"
	"time"
)

const (
	defaultStorageClass = "ipfs-csi"
	defaultNamespace    = "default"
	defaultDriverName   = "ipfs.csi.ptrvsrg.github.io"
)

// Config is test runtime configuration loaded from environment variables.
type Config struct {
	Run           bool
	StorageClass  string
	Namespace     string
	DriverName    string
	Timeout       time.Duration
	PollInterval  time.Duration
	KindContext   string
	RetryAttempts int
	RetryDelay    time.Duration
}

// LoadConfig reads all supported E2E_* settings.
func LoadConfig() Config {
	timeout := 180 * time.Second
	if s := os.Getenv("E2E_TIMEOUT"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			timeout = d
		}
	}
	pollInterval := 2 * time.Second
	if s := os.Getenv("E2E_POLL_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			pollInterval = d
		}
	}

	storageClass := os.Getenv("E2E_STORAGE_CLASS")
	if storageClass == "" {
		storageClass = defaultStorageClass
	}
	namespace := os.Getenv("E2E_NAMESPACE")
	if namespace == "" {
		namespace = defaultNamespace
	}
	driverName := os.Getenv("E2E_DRIVER_NAME")
	if driverName == "" {
		driverName = defaultDriverName
	}

	kindContext := os.Getenv("KIND_CONTEXT")
	if kindContext == "" {
		kindContext = "kind-csi-ipfs-e2e"
	}

	retryAttempts := 2
	if s := os.Getenv("E2E_RETRY_ATTEMPTS"); s != "" {
		if n, err := parsePositiveInt(s); err == nil {
			retryAttempts = n
		}
	}

	retryDelay := 3 * time.Second
	if s := os.Getenv("E2E_RETRY_DELAY"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			retryDelay = d
		}
	}

	run := os.Getenv("E2E_RUN") == "1" || os.Getenv("E2E_RUN") == "true"

	return Config{
		Run:           run,
		StorageClass:  storageClass,
		Namespace:     namespace,
		DriverName:    driverName,
		Timeout:       timeout,
		PollInterval:  pollInterval,
		KindContext:   kindContext,
		RetryAttempts: retryAttempts,
		RetryDelay:    retryDelay,
	}
}

func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n < 1 {
		return 0, os.ErrInvalid
	}
	return n, nil
}
