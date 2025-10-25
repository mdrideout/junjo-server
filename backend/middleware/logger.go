package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// SlogLogger returns an Echo middleware that logs HTTP requests using slog.
// It logs with different levels based on the response status code:
// - Health/ping endpoints (2xx): DEBUG
// - Other 2xx/3xx: INFO
// - 4xx: WARN
// - 5xx: ERROR
func SlogLogger(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()
			res := c.Response()

			// Process request
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			// Calculate latency
			latency := time.Since(start)

			// Prepare log fields
			fields := []any{
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", res.Status),
				slog.Duration("latency", latency),
				slog.String("remote_ip", c.RealIP()),
			}

			// Add error if present
			if err != nil {
				fields = append(fields, slog.String("error", err.Error()))
			}

			// Log with appropriate level based on status code and path
			status := res.Status
			path := req.URL.Path
			msg := "http request"

			// Health/monitoring endpoints at DEBUG level
			isHealthCheck := path == "/ping" || path == "/health"

			if status >= 500 {
				log.Error(msg, fields...)
			} else if status >= 400 {
				log.Warn(msg, fields...)
			} else if isHealthCheck {
				log.Debug(msg, fields...)
			} else {
				log.Info(msg, fields...)
			}

			return nil
		}
	}
}
