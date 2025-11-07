package main

import (
	"context"
	"crypto"
	"net/http"
	"os"

	"github.com/18F/hmacauth"
	"github.com/sirupsen/logrus"
)

var oauthSignatureHeaders = []string{
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

const gapSignatureHeader = "GAP-Signature"

type contextKey string

const userContextKey contextKey = "user"

//TODO: should we also store a list of components that the user is an admin of on the context? we would only have to compute it once

// GetUserFromContext retrieves the authenticated user from the request context.
func GetUserFromContext(ctx context.Context) (string, bool) {
	user, ok := ctx.Value(userContextKey).(string)
	return user, ok
}

func newAuthMiddleware(logger *logrus.Logger, hmacSecret []byte, next http.Handler) http.Handler {
	// Create HmacAuth instance with the same headers that oauth-proxy uses
	// These are the headers that oauth-proxy includes in the signature
	hmacAuth := hmacauth.NewHmacAuth(crypto.SHA256, hmacSecret, gapSignatureHeader, oauthSignatureHeaders)
	return authMiddleware(next, logger, hmacAuth)
}

func authMiddleware(next http.Handler, logger *logrus.Logger, hmacAuth hmacauth.HmacAuth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In development mode, we skip authentication
		if os.Getenv("DEV_MODE") == "1" {
			logger.Info("Skipping authentication in development mode")
			ctx := context.WithValue(r.Context(), userContextKey, "developer")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		authLogger := logger.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		})

		user := r.Header.Get("X-Forwarded-User")

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
