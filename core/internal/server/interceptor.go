// Package server - gRPC interceptors for authentication and logging
package server

import (
	"context"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/vault"
)

// SessionKey is the context key for session ID
type sessionKeyType struct{}

var sessionKey = sessionKeyType{}

// publicMethods are methods that don't require authentication
var publicMethods = map[string]bool{
	"/xswallet.WalletService/InitializeVault": true,
	"/xswallet.WalletService/UnlockVault":     true,
	"/xswallet.WalletService/GetVaultStatus":  true,
	"/xswallet.NodeService/GetPlatformCapabilities": true,
}

// NewAuthInterceptor creates a unary interceptor that validates session tokens
func NewAuthInterceptor(v *vault.Vault) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for public methods
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Extract session from metadata
		sessionID, err := extractSession(ctx)
		if err != nil {
			return nil, err
		}

		// Validate session
		if err := v.RequireSession(sessionID); err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
		}

		// Add session to context
		ctx = context.WithValue(ctx, sessionKey, sessionID)

		return handler(ctx, req)
	}
}

// NewStreamAuthInterceptor creates a stream interceptor that validates session tokens
func NewStreamAuthInterceptor(v *vault.Vault) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Skip auth for public methods
		if publicMethods[info.FullMethod] {
			return handler(srv, ss)
		}

		// Extract session from metadata
		sessionID, err := extractSession(ss.Context())
		if err != nil {
			return err
		}

		// Validate session
		if err := v.RequireSession(sessionID); err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
		}

		return handler(srv, ss)
	}
}

// extractSession extracts the session ID from gRPC metadata
// Expected format: authorization: Bearer <session_id>
func extractSession(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	auth := authHeaders[0]
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	sessionID := strings.TrimPrefix(auth, "Bearer ")
	if sessionID == "" {
		return "", status.Error(codes.Unauthenticated, "empty session ID")
	}

	return sessionID, nil
}

// GetSessionFromContext retrieves the session ID from the context
func GetSessionFromContext(ctx context.Context) string {
	if v := ctx.Value(sessionKey); v != nil {
		return v.(string)
	}
	return ""
}

// LoggingInterceptor logs all gRPC calls
func LoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	log.Printf("gRPC: %s", info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		log.Printf("gRPC: %s error: %v", info.FullMethod, err)
	}
	return resp, err
}

// StreamLoggingInterceptor logs all gRPC stream calls
func StreamLoggingInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	log.Printf("gRPC Stream: %s", info.FullMethod)
	err := handler(srv, ss)
	if err != nil {
		log.Printf("gRPC Stream: %s error: %v", info.FullMethod, err)
	}
	return err
}
