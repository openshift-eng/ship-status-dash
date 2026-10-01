package main

import (
	"crypto"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"ship-status-dash/pkg/auth"

	"github.com/18F/hmacauth"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func testProxy(t *testing.T, secret []byte, upstream http.Handler) http.Handler {
	t.Helper()
	upstreamSrv := httptest.NewServer(upstream)
	t.Cleanup(upstreamSrv.Close)
	upstreamURL, err := url.Parse(upstreamSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Users: []User{{
		Username:     "developer",
		PasswordHash: string(hash),
		Email:        "developer@example.com",
	}}}
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	signer := hmacauth.NewHmacAuth(crypto.SHA256, secret, auth.GAPSignatureHeader, auth.OAuthSignatureHeaders)
	return basicAuthHandler(cfg, upstreamURL, signer, secret, logger, "http://localhost:3030")
}

func TestDevSessionCookieAuthenticatesDelete(t *testing.T) {
	secret := []byte("test-secret")
	var gotUser string
	var result hmacauth.AuthenticationResult
	signer := hmacauth.NewHmacAuth(crypto.SHA256, secret, auth.GAPSignatureHeader, auth.OAuthSignatureHeaders)
	handler := testProxy(t, secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Forwarded-User")
		result, _, _ = signer.AuthenticateRequest(r)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/teams/TRT/slo/items/payload_streams/tag/links/31", nil)
	req.AddCookie(&http.Cookie{Name: devSessionCookie, Value: devSessionValue(secret, "developer")})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotUser != "developer" {
		t.Fatalf("forwarded user = %q", gotUser)
	}
	if result != hmacauth.ResultMatch {
		t.Fatalf("HMAC result = %d, want %d", result, hmacauth.ResultMatch)
	}
}

func TestTamperedDevSessionCookieIsRejected(t *testing.T) {
	secret := []byte("test-secret")
	handler := testProxy(t, secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called")
		w.WriteHeader(http.StatusOK)
	}))

	value := devSessionValue(secret, "developer")
	req := httptest.NewRequest(http.MethodDelete, "/api/x", nil)
	req.AddCookie(&http.Cookie{Name: devSessionCookie, Value: value + "x"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate")
	}
}

func TestBasicAuthWithoutCookie(t *testing.T) {
	secret := []byte("test-secret")
	var gotUser string
	handler := testProxy(t, secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Forwarded-User")
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	req.SetBasicAuth("developer", "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotUser != "developer" {
		t.Fatalf("forwarded user = %q", gotUser)
	}
}

func TestLoginSetsDevSessionCookie(t *testing.T) {
	secret := []byte("test-secret")
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Users: []User{{
		Username:     "developer",
		PasswordHash: string(hash),
		Email:        "developer@example.com",
	}}}
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	handler := oauthStartHandler(cfg, logger, secret)

	req := httptest.NewRequest(http.MethodGet, "/oauth/start", nil)
	req.SetBasicAuth("developer", "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d", rec.Code)
	}
	cookie := rec.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != devSessionCookie {
		t.Fatalf("cookies = %#v", cookie)
	}
	if !cookie[0].HttpOnly || cookie[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags = httpOnly %v sameSite %v", cookie[0].HttpOnly, cookie[0].SameSite)
	}
	username, ok := usernameFromDevSession(secret, cookie[0].Value)
	if !ok || username != "developer" {
		t.Fatalf("cookie user = %q ok=%v", username, ok)
	}
}
