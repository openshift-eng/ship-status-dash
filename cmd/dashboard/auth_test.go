package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	hmacSecret := []byte("test-secret-key")
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("non-mutating requests pass through", func(t *testing.T) {
		tests := []struct {
			method string
		}{
			{method: http.MethodGet},
			{method: http.MethodHead},
			{method: http.MethodOptions},
			{method: http.MethodTrace},
		}

		for _, tt := range tests {
			t.Run(tt.method, func(t *testing.T) {
				handlerCalled := false
				nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handlerCalled = true
					w.WriteHeader(http.StatusOK)
				})

				middleware := authMiddleware(nextHandler, logger, hmacSecret)
				req := httptest.NewRequest(tt.method, "/test", nil)
				w := httptest.NewRecorder()

				middleware.ServeHTTP(w, req)

				assert.True(t, handlerCalled, "handler should be called for non-mutating requests")
				assert.Equal(t, http.StatusOK, w.Code)
			})
		}
	})

	t.Run("mutating requests without authentication fail", func(t *testing.T) {
		tests := []struct {
			name            string
			method          string
			userHeader      string
			signatureHeader string
			expectedStatus  int
			expectedBody    string
		}{
			{
				name:            "missing X-Forwarded-User header",
				method:          http.MethodPost,
				signatureHeader: "some-signature",
				expectedStatus:  http.StatusUnauthorized,
				expectedBody:    "Missing X-Forwarded-User header\n",
			},
			{
				name:           "missing X-Signature header",
				method:         http.MethodPost,
				userHeader:     "test-user",
				expectedStatus: http.StatusUnauthorized,
				expectedBody:   "Missing X-Signature header\n",
			},
			{
				name:            "invalid hex signature format",
				method:          http.MethodPost,
				userHeader:      "test-user",
				signatureHeader: "not-valid-hex!",
				expectedStatus:  http.StatusUnauthorized,
				expectedBody:    "Invalid signature format\n",
			},
			{
				name:            "invalid HMAC signature",
				method:          http.MethodPost,
				userHeader:      "test-user",
				signatureHeader: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				expectedStatus:  http.StatusUnauthorized,
				expectedBody:    "Invalid signature\n",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				os.Unsetenv("DEV_MODE")

				handlerCalled := false
				nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handlerCalled = true
				})

				middleware := authMiddleware(nextHandler, logger, hmacSecret)
				req := httptest.NewRequest(tt.method, "/test", nil)

				if tt.userHeader != "" {
					req.Header.Set("X-Forwarded-User", tt.userHeader)
				}
				if tt.signatureHeader != "" {
					req.Header.Set("X-Signature", tt.signatureHeader)
				}

				w := httptest.NewRecorder()
				middleware.ServeHTTP(w, req)

				assert.False(t, handlerCalled, "handler should not be called when authentication fails")
				assert.Equal(t, tt.expectedStatus, w.Code)
				assert.Equal(t, tt.expectedBody, w.Body.String())
			})
		}
	})

	t.Run("valid authentication succeeds", func(t *testing.T) {
		os.Unsetenv("DEV_MODE")

		tests := []struct {
			name   string
			method string
			user   string
		}{
			{name: "POST with valid signature", method: http.MethodPost, user: "test-user"},
			{name: "PATCH with valid signature", method: http.MethodPatch, user: "test-user"},
			{name: "DELETE with valid signature", method: http.MethodDelete, user: "test-user"},
			{name: "POST with different user", method: http.MethodPost, user: "another-user"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mac := hmac.New(sha256.New, hmacSecret)
				mac.Write([]byte(tt.user))
				validSignature := hex.EncodeToString(mac.Sum(nil))

				var receivedUser string
				handlerCalled := false
				nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handlerCalled = true
					user, ok := GetUserFromContext(r.Context())
					require.True(t, ok, "user should be in context")
					receivedUser = user
					w.WriteHeader(http.StatusOK)
				})

				middleware := authMiddleware(nextHandler, logger, hmacSecret)
				req := httptest.NewRequest(tt.method, "/test", nil)
				req.Header.Set("X-Forwarded-User", tt.user)
				req.Header.Set("X-Signature", validSignature)

				w := httptest.NewRecorder()
				middleware.ServeHTTP(w, req)

				assert.True(t, handlerCalled, "handler should be called with valid authentication")
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, tt.user, receivedUser, "user from context should match header")
			})
		}
	})

	t.Run("DEV_MODE bypasses authentication", func(t *testing.T) {
		tests := []struct {
			name         string
			method       string
			userHeader   string
			expectedUser string
		}{
			{
				name:         "POST with user header",
				method:       http.MethodPost,
				userHeader:   "test-user",
				expectedUser: "test-user",
			},
			{
				name:         "POST without user header",
				method:       http.MethodPost,
				expectedUser: "developer",
			},
			{
				name:         "PATCH with user header",
				method:       http.MethodPatch,
				userHeader:   "custom-user",
				expectedUser: "custom-user",
			},
			{
				name:         "DELETE without user header",
				method:       http.MethodDelete,
				expectedUser: "developer",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				os.Setenv("DEV_MODE", "1")
				defer os.Unsetenv("DEV_MODE")

				var receivedUser string
				handlerCalled := false
				nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handlerCalled = true
					user, ok := GetUserFromContext(r.Context())
					require.True(t, ok, "user should be in context")
					receivedUser = user
					w.WriteHeader(http.StatusOK)
				})

				middleware := authMiddleware(nextHandler, logger, hmacSecret)
				req := httptest.NewRequest(tt.method, "/test", nil)

				if tt.userHeader != "" {
					req.Header.Set("X-Forwarded-User", tt.userHeader)
				}

				w := httptest.NewRecorder()
				middleware.ServeHTTP(w, req)

				assert.True(t, handlerCalled, "handler should be called in DEV_MODE")
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, tt.expectedUser, receivedUser, "user should match expected")
			})
		}
	})

	t.Run("DEV_MODE bypasses signature validation", func(t *testing.T) {
		os.Setenv("DEV_MODE", "1")
		defer os.Unsetenv("DEV_MODE")

		handlerCalled := false
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		middleware := authMiddleware(nextHandler, logger, hmacSecret)
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("X-Forwarded-User", "test-user")
		req.Header.Set("X-Signature", "invalid-signature-without-validation")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		assert.True(t, handlerCalled, "handler should be called even with invalid signature in DEV_MODE")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetUserFromContext(t *testing.T) {
	t.Run("user in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userContextKey, "test-user")
		user, ok := GetUserFromContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, "test-user", user)
	})

	t.Run("user not in context", func(t *testing.T) {
		ctx := context.Background()
		user, ok := GetUserFromContext(ctx)

		assert.False(t, ok)
		assert.Empty(t, user)
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userContextKey, 123)
		user, ok := GetUserFromContext(ctx)

		assert.False(t, ok)
		assert.Empty(t, user)
	})
}
