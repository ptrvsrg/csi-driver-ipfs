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

// Package middleware provides gRPC server interceptors for the CSI driver.
package middleware

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// LoggingUnaryServerInterceptor installs a unary server interceptor that logs request metadata, peer address,
// RPC method, gRPC status code, and duration for each call.
//
// l is the logger used for start/finish lines. The returned interceptor passes through ctx, req, and handler
// unchanged aside from logging side effects. It returns the handler response and error verbatim.
func LoggingUnaryServerInterceptor(l zerolog.Logger) grpc.UnaryServerInterceptor {
	log.Debug().Msg("setup logging unary middleware")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		p, _ := peer.FromContext(ctx)
		addr := "unknown"
		if p != nil && p.Addr != nil {
			addr = p.Addr.String()
		}

		l.Debug().
			Str("addr", addr).
			Str("method", info.FullMethod).
			Msg("start unary call")

		resp, err := handler(ctx, req)
		code := status.Code(err)

		l.Debug().
			Str("addr", addr).
			Str("method", info.FullMethod).
			Str("code", code.String()).
			Dur("duration", time.Since(start)).
			Msg("finish unary call")

		return resp, err
	}
}
