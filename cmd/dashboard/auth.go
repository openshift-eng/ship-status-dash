package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
)

type contextKey string

const userContextKey contextKey = "user"

// GetUserFromContext retrieves the authenticated user from the request context.
func GetUserFromContext(ctx context.Context) (string, bool) {
	user, ok := ctx.Value(userContextKey).(string)
	return user, ok
}

func authMiddleware(next http.Handler, logger *logrus.Logger, hmacSecret []byte) http.Handler {
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
		signature := r.Header.Get("X-Signature")

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

		if signature == "" {
			authLogger.Warn("Missing X-Signature header")
			http.Error(w, "Missing X-Signature header", http.StatusUnauthorized)
			return
		}

		mac := hmac.New(sha256.New, hmacSecret)
		mac.Write([]byte(user))
		expectedSignatureBytes := mac.Sum(nil)

		signatureBytes, err := hex.DecodeString(signature)
		if err != nil {
			authLogger.Warn("Invalid signature format (not hex-encoded)")
			http.Error(w, "Invalid signature format", http.StatusUnauthorized)
			return
		}

		if !hmac.Equal(signatureBytes, expectedSignatureBytes) {
			authLogger.Warn("Invalid HMAC signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
