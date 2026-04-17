// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middleware

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

func TestLoggingUnaryServerInterceptor_NoPeerInContext(t *testing.T) {
	t.Parallel()

	interceptor := LoggingUnaryServerInterceptor(zerolog.Nop())
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(
		context.Background(),
		"struct-req",
		&grpc.UnaryServerInfo{FullMethod: "/csi.v1.Controller/CreateVolume"},
		handler,
	)
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}
