package api

import (
	"context"
	"github.com/frodejac/globster/internal/random"
	"log/slog"
	"net/http"
	"time"
)

type RequestIDKey struct{}

// RequestIdMiddleware adds a unique request ID to each request
func RequestIdMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = random.HexString(16)
			r.Header.Set("X-Request-ID", requestID)
		}
		r = r.WithContext(context.WithValue(r.Context(), RequestIDKey{}, requestID))
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleWare logs the incoming requests
func LoggingMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()
		slog.Info(
			"Request received",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.Any("request_id", r.Context().Value(RequestIDKey{})),
		)
		next.ServeHTTP(w, r)
		slog.Info(
			"Request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.Any("request_id", r.Context().Value(RequestIDKey{})),
			slog.Duration("duration", time.Since(t0)),
		)
	})
}

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware(useHsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content Security Policy
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; object-src 'none'; img-src 'self'; style-src 'self'; connect-src 'self'; font-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'self';")

			// Prevent browsers from MIME-sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			if useHsts {
				// HTTP Strict Transport Security (HSTS)
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}
			// Referrer Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions Policy
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=(), interest-cohort=()")

			// XSS Protection (legacy, but still useful for older browsers)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}
