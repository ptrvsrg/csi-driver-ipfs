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
