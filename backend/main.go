package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"context"
	"junjo-server/api"
	"junjo-server/api/internal_auth"
	"junjo-server/api_keys"
	"junjo-server/auth"
	"junjo-server/db"
	"junjo-server/db_duckdb"
	"junjo-server/db_gen"
	"junjo-server/ingestion_client"
	"junjo-server/logger"
	m "junjo-server/middleware"
	pb "junjo-server/proto_gen"
	"junjo-server/telemetry"
	u "junjo-server/utils"
	"net"
	"time"

	"github.com/gorilla/sessions"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/joho/godotenv"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		// Just print to stderr before logger is initialized
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Initialize logger
	log := logger.InitLogger()
	log.Info("starting junjo backend service")

	// Host
	port := "1323"
	host := "0.0.0.0"
	serverHostPort := host + ":" + port

	// SQLite DB
	db.Connect()
	defer db.Close()

	// DuckDB
	duck_err := db_duckdb.Connect()
	if duck_err != nil {
		log.Error("failed to connect to duckdb", slog.Any("error", duck_err))
		os.Exit(1)
	}
	defer db_duckdb.Close()

	// Ingestion Client
	ingestionClient, err := ingestion_client.NewClient()
	if err != nil {
		log.Error("failed to create ingestion client", slog.Any("error", err))
		os.Exit(1)
	}
	defer ingestionClient.Close()

	// Start a background goroutine to poll for spans
	go func() {
		ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
		defer ticker.Stop()

		queries := db_gen.New(db.DB)
		var lastKey []byte
		// At startup, try to load the last processed key from the database.
		retrievedKey, err := queries.GetPollerState(context.Background())
		if err != nil && err != sql.ErrNoRows {
			log.Error("failed to load poller state", slog.Any("error", err))
			os.Exit(1)
		} else if err == sql.ErrNoRows {
			log.Info("no previous poller state found, starting from beginning")
		} else {
			lastKey = retrievedKey
			log.Info("resuming poller", slog.String("last_key", fmt.Sprintf("%x", lastKey)))
		}

		for range ticker.C {
			log.Debug("polling for new spans")
			spans, err := ingestionClient.ReadSpans(context.Background(), lastKey, 100)
			if err != nil {
				log.Error("error reading spans", slog.Any("error", err))
				continue
			}

			if len(spans) > 0 {
				lastKey = spans[len(spans)-1].KeyUlid
				log.Info("received spans", slog.Int("count", len(spans)), slog.String("last_key", fmt.Sprintf("%x", lastKey)))
				var processedSpans []*tracepb.Span
				for _, receivedSpan := range spans {
					var span tracepb.Span
					if err := proto.Unmarshal(receivedSpan.SpanBytes, &span); err != nil {
						log.Warn("error unmarshaling span", slog.Any("error", err))
						continue // Skip to the next span
					}
					processedSpans = append(processedSpans, &span)
				}

				if len(processedSpans) > 0 {
					// Extract the service name from the first span's resource
					// All spans in a batch should have the same service name
					var serviceName string
					if len(spans) > 0 {
						// Unmarshal the resource bytes
						var resource resourcepb.Resource
						if err := proto.Unmarshal(spans[0].ResourceBytes, &resource); err != nil {
							log.Warn("error unmarshaling resource", slog.Any("error", err))
							serviceName = "NO_SERVICE_NAME"
						} else {
							// Extract service name from resource attributes
							for _, attr := range resource.Attributes {
								if attr.Key == "service.name" {
									if stringValue, ok := attr.Value.Value.(*commonpb.AnyValue_StringValue); ok {
										serviceName = stringValue.StringValue
										break
									}
								}
							}
							if serviceName == "" {
								serviceName = "NO_SERVICE_NAME"
							}
						}
					} else {
						serviceName = "NO_SERVICE_NAME"
					}

					if err := telemetry.BatchProcessSpans(context.Background(), serviceName, processedSpans); err != nil {
						log.Error("error processing spans batch", slog.Any("error", err))
					} else {
						// If the batch was processed successfully, update the last key in the database.
						err := queries.UpsertPollerState(context.Background(), lastKey)
						if err != nil {
							log.Error("failed to save poller state", slog.Any("error", err))
						}
					}
				}
			} else {
				log.Debug("no new spans found")
			}
		}
	}()

	// Initialize Echo
	e := echo.New()
	log.Info("initialized echo server", slog.String("host_port", serverHostPort))
	e.Validator = u.NewCustomValidator()

	// Middleware
	e.Pre(middleware.Recover()) // Recover must be first
	e.Use(m.SlogLogger(log))

	// CORS middleware
	// Must be registered with `Pre` to run before the router, which allows it to handle
	// OPTIONS requests for routes that don't have an explicit OPTIONS handler.
	allowedOriginsEnv := os.Getenv("JUNJO_ALLOW_ORIGINS")
	config := middleware.CORSConfig{
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderXCSRFToken},
		AllowCredentials: true,
	}

	// AllowOriginFunc is a custom function to validate the origin.
	// It's used here to provide more robust logging and explicit control over the CORS logic.
	// According to Echo docs, if this option is set, the AllowOrigins array is ignored.
	config.AllowOriginFunc = func(origin string) (bool, error) {
		allowedOriginsEnv := os.Getenv("JUNJO_ALLOW_ORIGINS")
		if len(allowedOriginsEnv) == 0 {
			log.Info("cors check: allowing origin for local dev", slog.String("origin", origin))
			return true, nil
		}

		allowedOrigins := strings.Split(allowedOriginsEnv, ",")
		for _, allowed := range allowedOrigins {
			trimmedAllowed := strings.TrimSpace(allowed)
			if trimmedAllowed == origin {
				log.Info("cors check: allowing origin (exact match)", slog.String("origin", origin))
				return true, nil
			}
		}

		log.Warn("cors check: denying origin", slog.String("origin", origin), slog.String("reason", "not in JUNJO_ALLOW_ORIGINS"))
		return false, nil
	}

	// Log the configured origins for clarity on startup
	if len(allowedOriginsEnv) > 0 {
		log.Info("cors allowed origins configured", slog.String("origins", allowedOriginsEnv))
	} else {
		log.Info("cors allowed origins not set, reflecting any origin")
	}
	e.Pre(middleware.CORSWithConfig(config))

	// Session Middleware
	sessionSecret := os.Getenv("JUNJO_SESSION_SECRET")
	if sessionSecret == "" {
		log.Error("JUNJO_SESSION_SECRET environment variable is not set")
		os.Exit(1)
	}
	e.Use(session.Middleware(sessions.NewCookieStore([]byte(sessionSecret))))

	// CSRF Middleware (Echo's built-in CSRF)
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "header:X-CSRF-Token,cookie:csrf", // Look in header AND cookie
		CookieName:     "csrf",
		CookieSecure:   true, // HTTPS in production
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		ContextKey:     "csrf", // Key to access CSRF token in handlers
		Skipper: func(c echo.Context) bool {
			if c.Request().Method == http.MethodOptions || c.Path() == "/sign-in" {
				return true // Skip CSRF check for OPTIONS and /sign-in
			}
			return false
		},
	}))

	// Auth Middleware
	e.Use(m.Auth()) // Auth guard all routes by default

	// ROUTES
	auth.InitRoutes(e)
	api.InitRoutes(e)
	api_keys.InitRoutes(e)

	// Ping route
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	// --- Internal gRPC Server Setup ---
	go func() {
		internalGrpcAddr := ":50053"
		lis, err := net.Listen("tcp", internalGrpcAddr)
		if err != nil {
			log.Error("failed to listen for internal grpc", slog.Any("error", err))
			os.Exit(1)
		}

		grpcServer := grpc.NewServer(
			grpc.UnaryInterceptor(
				grpc_logging.UnaryServerInterceptor(
					logger.InterceptorLogger(log),
					grpc_logging.WithLogOnEvents(grpc_logging.FinishCall),
				),
			),
		)
		internalAuthSvc := internal_auth.NewInternalAuthService()
		pb.RegisterInternalAuthServiceServer(grpcServer, internalAuthSvc)

		log.Info("internal grpc server listening", slog.String("address", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("failed to serve internal grpc", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Start the server
	e.Logger.Fatal(e.Start(serverHostPort))
}
