package middleware

import (
	"net/http"
	"task-orchestrator/logger"
	"time"
)

// responseWriterInterceptor is a wrapper around http.ResponseWriter that allows us to capture the status code.
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriterInterceptor creates a new responseWriterInterceptor.
// It defaults the statusCode to 200, as WriteHeader is not always called.
func newResponseWriterInterceptor(w http.ResponseWriter) *responseWriterInterceptor {
	return &responseWriterInterceptor{w, http.StatusOK}
}

// WriteHeader captures the status code and calls the original WriteHeader.
func (rwi *responseWriterInterceptor) WriteHeader(code int) {
	rwi.statusCode = code
	rwi.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware creates a new HTTP middleware for logging requests and responses.
func LoggingMiddleware(lg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// Wrap the original ResponseWriter to capture the status code
			rwi := newResponseWriterInterceptor(w)

			// Call the next handler in the chain with our wrapped ResponseWriter
			next.ServeHTTP(rwi, r)

			duration := time.Since(startTime)

			// Use the existing logger.HTTP method to log the request details
			lg.HTTP(
				r.Method,
				r.URL.Path,
				rwi.statusCode,
				duration,
				map[string]any{
					"remote_addr": r.RemoteAddr,
					"user_agent":  r.UserAgent(),
				},
			)
		})
	}
}
