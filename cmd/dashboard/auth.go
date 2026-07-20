package main

import (
	"context"
	"crypto"
	"net/http"
	"os"
	"slices"
	"strings"

	"ship-status-dash/pkg/auth"
	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/types"

	"github.com/18F/hmacauth"
	"github.com/sirupsen/logrus"
)

type contextKey string

const userContextKey contextKey = "user"

const actingForHeader = "X-Acting-For"

// GetUserFromContext retrieves the authenticated user from the request context.
// For trusted delegators, this returns the delegated (acting_for) identity.
func GetUserFromContext(ctx context.Context) (string, bool) {
	user, ok := ctx.Value(userContextKey).(string)
	return user, ok
}

func newAuthMiddleware(logger *logrus.Logger, hmacSecret []byte, configManager *config.Manager[types.DashboardConfig], next http.Handler) http.Handler {
	hmacAuth := hmacauth.NewHmacAuth(crypto.SHA256, hmacSecret, auth.GAPSignatureHeader, auth.OAuthSignatureHeaders)
	return authMiddleware(next, logger, hmacAuth, configManager)
}

func authMiddleware(next http.Handler, logger *logrus.Logger, hmacAuth hmacauth.HmacAuth, configManager *config.Manager[types.DashboardConfig]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("SKIP_AUTH") == "1" {
			logger.Info("Skipping authentication in development mode (SKIP_AUTH is set)")
			user := "developer"
			resolved := resolveDelegation(user, r, configManager, logger)
			if resolved == "" {
				http.Error(w, "acting_for is required for delegated requests", http.StatusBadRequest)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, resolved)
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

		resolved := resolveDelegation(user, r, configManager, authLogger)
		if resolved == "" {
			http.Error(w, "acting_for is required for delegated requests", http.StatusBadRequest)
			return
		}
		if resolved != user {
			authLogger.WithField("acting_for", resolved).Info("Delegated request")
		}

		ctx := context.WithValue(r.Context(), userContextKey, resolved)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveDelegation checks whether the authenticated user is a trusted delegator.
// If so, it requires X-Acting-For and returns that identity. If the header is missing,
// it returns "" to signal a 400. Non-delegators pass through unchanged.
func resolveDelegation(authenticatedUser string, r *http.Request, configManager *config.Manager[types.DashboardConfig], logger logrus.FieldLogger) string {
	cfg := configManager.Get()
	if !slices.Contains(cfg.TrustedDelegators, authenticatedUser) {
		return authenticatedUser
	}
	actingFor := strings.TrimSpace(r.Header.Get(actingForHeader))
	if actingFor == "" {
		logger.Warn("Trusted delegator did not provide X-Acting-For header")
		return ""
	}
	return actingFor
}
