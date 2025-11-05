package main

import (
	"context"
	"crypto"
	"net/http"
	"os"

	"github.com/18F/hmacauth"
	"github.com/sirupsen/logrus"
)

type contextKey string

const userContextKey contextKey = "user"

// GetUserFromContext retrieves the authenticated user from the request context.
func GetUserFromContext(ctx context.Context) (string, bool) {
	user, ok := ctx.Value(userContextKey).(string)
	return user, ok
}

func newAuthMiddleware(logger *logrus.Logger, hmacSecret []byte, next http.Handler) http.Handler {
	// Create HmacAuth instance with the same headers that oauth-proxy uses
	// These are the headers that oauth-proxy includes in the signature
	signatureHeaders := []string{
		"Content-Length",
		"Content-Md5",
		"Content-Type",
		"Date",
		"Authorization",
		"X-Forwarded-User",
		"X-Forwarded-Email",
		"X-Forwarded-Access-Token",
		"Cookie",
		"Gap-Auth",
	}
	hmacAuth := hmacauth.NewHmacAuth(crypto.SHA256, hmacSecret, "GAP-Signature", signatureHeaders)
	return authMiddleware(next, logger, hmacAuth)
}

func authMiddleware(next http.Handler, logger *logrus.Logger, hmacAuth hmacauth.HmacAuth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isMutating := r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodDelete

		if !isMutating {
			next.ServeHTTP(w, r)
			return
		}

		authLogger := logger.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		})

		user := r.Header.Get("X-Forwarded-User")

		if os.Getenv("DEV_MODE") == "1" {
			if user == "" {
				user = "developer"
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if user == "" {
			authLogger.Warn("Missing X-Forwarded-User header")
			http.Error(w, "Missing X-Forwarded-User header", http.StatusUnauthorized)
			return
		}

		authLogger = authLogger.WithField("user", user)
		result, _, _ := hmacAuth.AuthenticateRequest(r)
		switch result {
		case hmacauth.ResultNoSignature:
			authLogger.Warn("Missing GAP-Signature header")
			http.Error(w, "Missing GAP-Signature header", http.StatusUnauthorized)
			return
		case hmacauth.ResultInvalidFormat:
			authLogger.Warn("Invalid signature format")
			http.Error(w, "Invalid signature format", http.StatusUnauthorized)
			return
		case hmacauth.ResultUnsupportedAlgorithm:
			authLogger.Warn("Unsupported signature algorithm")
			http.Error(w, "Unsupported signature algorithm", http.StatusUnauthorized)
			return
		case hmacauth.ResultMismatch:
			authLogger.Warn("Invalid HMAC signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		case hmacauth.ResultMatch:
			// Signature is valid, continue
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
