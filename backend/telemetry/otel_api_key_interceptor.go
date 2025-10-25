package telemetry

import (
	"context"
	"database/sql" // Needed for sql.ErrNoRows
	"log/slog"

	"junjo-server/api_keys" // Import your api_keys repository

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ApiKeyAuthInterceptor checks for a valid API key in the request metadata.
func ApiKeyAuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Log the incoming request
	slog.DebugContext(ctx, "api key auth interceptor: received request", slog.String("method", info.FullMethod))

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		slog.WarnContext(ctx, "api key auth interceptor: missing metadata")
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	// Check for the API key header (adjust key name if you used something else)
	apiKeys := md.Get("x-api-key")
	if len(apiKeys) == 0 {
		slog.WarnContext(ctx, "api key auth interceptor: missing x-api-key header")
		return nil, status.Errorf(codes.Unauthenticated, "API key is required")
	}

	apiKey := apiKeys[0]
	if apiKey == "" {
		slog.WarnContext(ctx, "api key auth interceptor: empty x-api-key header")
		return nil, status.Errorf(codes.Unauthenticated, "API key cannot be empty")
	}

	// Validate the API key against the database
	_, err := api_keys.GetAPIKey(ctx, apiKey)
	if err != nil {
		if err == sql.ErrNoRows {
			slog.WarnContext(ctx, "api key auth interceptor: invalid api key provided", slog.String("api_key_prefix", apiKey[:min(8, len(apiKey))]))
			return nil, status.Errorf(codes.Unauthenticated, "invalid API key")
		}
		// Handle other potential database errors
		slog.ErrorContext(ctx, "api key auth interceptor: error validating api key", slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "error validating API key")
	}

	// API key is valid, proceed with the original handler
	slog.InfoContext(ctx, "api key auth interceptor: api key validated successfully", slog.String("api_key_prefix", apiKey[:min(8, len(apiKey))]))
	return handler(ctx, req)
}

// Helper function to avoid index out of range
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
