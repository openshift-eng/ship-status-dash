package main

import (
	"context"
	"crypto"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ship-status-dash/pkg/auth"

	"github.com/18F/hmacauth"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

type User struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
	Email        string `yaml:"email"`
}

type Config struct {
	Users []User `yaml:"users"`
}

type Options struct {
	ConfigPath     string
	Port           string
	Upstream       string
	HMACSecretFile string
}

func NewOptions() *Options {
	opts := &Options{}
	flag.StringVar(&opts.ConfigPath, "config", "", "Path to config file")
	flag.StringVar(&opts.Port, "port", "8443", "Port to listen on")
	flag.StringVar(&opts.Upstream, "upstream", "", "Upstream server URL")
	flag.StringVar(&opts.HMACSecretFile, "hmac-secret-file", "", "File containing HMAC secret")
	flag.Parse()
	return opts
}

func (o *Options) Validate() error {
	if o.ConfigPath == "" {
		return errors.New("config path is required (use --config flag)")
	}
	if _, err := os.Stat(o.ConfigPath); os.IsNotExist(err) {
		return errors.New("config file does not exist: " + o.ConfigPath)
	}
	if o.Upstream == "" {
		return errors.New("upstream URL is required (use --upstream flag)")
	}
	if o.HMACSecretFile == "" {
		return errors.New("hmac secret file is required (use --hmac-secret-file flag)")
	}
	if _, err := os.Stat(o.HMACSecretFile); os.IsNotExist(err) {
		return errors.New("hmac secret file does not exist: " + o.HMACSecretFile)
	}
	return nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func getHMACSecret(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read HMAC secret file: %w", err)
	}
	return []byte(strings.TrimSpace(string(data))), nil
}

func authenticateUser(username, password string, config *Config) (*User, error) {
	for _, user := range config.Users {
		if user.Username == username {
			err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
			if err != nil {
				return nil, fmt.Errorf("invalid password")
			}
			return &user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func basicAuthHandler(config *Config, upstreamURL *url.URL, hmacAuth hmacauth.HmacAuth, logger *logrus.Logger) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		username, password, ok := req.BasicAuth()
		if !ok {
			return
		}

		user, err := authenticateUser(username, password, config)
		if err != nil {
			return
		}

		req.Header.Set("X-Forwarded-User", user.Username)
		req.Header.Set("X-Forwarded-Email", user.Email)
		req.Header.Set("X-Forwarded-Access-Token", "mock-access-token-"+user.Username)

		if req.Header.Get("Date") == "" {
			req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
		}

		if req.ContentLength > 0 {
			req.Header.Set("Content-Length", fmt.Sprintf("%d", req.ContentLength))
		}

		hmacAuth.SignRequest(req)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := authenticateUser(username, password, config)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"username": username,
				"error":    err,
			}).Warn("Authentication failed")
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		logger.WithFields(logrus.Fields{
			"username": user.Username,
			"path":     r.URL.Path,
		}).Debug("Authenticated user")

		proxy.ServeHTTP(w, r)
	})
}

func setupLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return log
}

func main() {
	logger := setupLogger()
	opts := NewOptions()

	if err := opts.Validate(); err != nil {
		logger.WithField("error", err).Fatal("Invalid options")
	}

	config, err := loadConfig(opts.ConfigPath)
	if err != nil {
		logger.WithField("error", err).Fatal("Failed to load config")
	}

	hmacSecret, err := getHMACSecret(opts.HMACSecretFile)
	if err != nil {
		logger.WithField("error", err).Fatal("Failed to load HMAC secret")
	}

	upstreamURL, err := url.Parse(opts.Upstream)
	if err != nil {
		logger.WithField("error", err).Fatal("Failed to parse upstream URL")
	}

	hmacAuth := hmacauth.NewHmacAuth(crypto.SHA256, hmacSecret, auth.GAPSignatureHeader, auth.OAuthSignatureHeaders)

	handler := basicAuthHandler(config, upstreamURL, hmacAuth, logger)

	server := &http.Server{
		Addr:    ":" + opts.Port,
		Handler: handler,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithFields(logrus.Fields{
				"port":  opts.Port,
				"error": err,
			}).Fatal("Server failed to start")
		}
	}()

	logger.WithFields(logrus.Fields{
		"port":     opts.Port,
		"upstream": opts.Upstream,
	}).Info("Mock oauth-proxy started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.WithField("error", err).Error("Server shutdown failed")
	}
}
