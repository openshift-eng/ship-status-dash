package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Options contains command-line configuration options for the dashboard server.
type Options struct {
	ConfigPath     string
	Port           string
	DatabaseDSN    string
	CORSOrigin     string
	HMACSecretFile string
}

// NewOptions parses command-line flags and returns a new Options instance.
func NewOptions() *Options {
	opts := &Options{}

	flag.StringVar(&opts.ConfigPath, "config", "", "Path to config file")
	flag.StringVar(&opts.Port, "port", "8080", "Port to listen on")
	flag.StringVar(&opts.DatabaseDSN, "dsn", "", "PostgreSQL DSN connection string")
	flag.StringVar(&opts.CORSOrigin, "cors-origin", "*", "Allowed CORS origin (use '*' for all origins)")
	flag.StringVar(&opts.HMACSecretFile, "hmac-secret-file", "", "File containing HMAC secret")
	flag.Parse()

	return opts
}

// Validate checks that all required options are provided and valid.
func (o *Options) Validate() error {
	if o.ConfigPath == "" {
		return errors.New("config path is required (use --config flag)")
	}

	if _, err := os.Stat(o.ConfigPath); os.IsNotExist(err) {
		return errors.New("config file does not exist: " + o.ConfigPath)
	}

	if o.Port == "" {
		return errors.New("port cannot be empty")
	}

	if o.DatabaseDSN == "" {
		return errors.New("database DSN is required (use --dsn flag)")
	}

	if o.HMACSecretFile == "" {
		return errors.New("hmac secret file is required (use --hmac-secret-file flag)")
	}
	if _, err := os.Stat(o.HMACSecretFile); os.IsNotExist(err) {
		return errors.New("hmac secret file does not exist: " + o.HMACSecretFile)
	}

	return nil
}

func setupLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return log
}

func loadConfig(log *logrus.Logger, configPath string) *types.Config {
	log.Infof("Loading config from %s", configPath)

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		log.WithFields(logrus.Fields{
			"config_path": configPath,
			"error":       err,
		}).Fatal("Failed to read config file")
	}

	var config types.Config
	if err := yaml.Unmarshal(configFile, &config); err != nil {
		log.WithFields(logrus.Fields{
			"config_path": configPath,
			"error":       err,
		}).Fatal("Failed to parse config file")
	}

	// We need to compute and store all the slugs to match by them later
	for _, component := range config.Components {
		component.Slug = utils.Slugify(component.Name)
		for i := range component.Subcomponents {
			component.Subcomponents[i].Slug = utils.Slugify(component.Subcomponents[i].Name)
		}
	}

	log.Infof("Loaded configuration with %d components", len(config.Components))
	return &config
}

func connectDatabase(log *logrus.Logger, dsn string) *gorm.DB {
	log.Info("Connecting to PostgreSQL database")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.WithField("error", err).Fatal("Failed to connect to database")
	}
	return db
}

func getHMACSecret(path string) []byte {
	secret, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	// Trim any trailing newlines/whitespace from the secret
	return []byte(strings.TrimSpace(string(secret)))
}

func main() {
	log := setupLogger()
	opts := NewOptions()

	if err := opts.Validate(); err != nil {
		log.WithField("error", err).Fatal("Invalid command-line options")
	}

	config := loadConfig(log, opts.ConfigPath)
	db := connectDatabase(log, opts.DatabaseDSN)
	hmacSecret := getHMACSecret(opts.HMACSecretFile)
	server := NewServer(config, db, log, opts.CORSOrigin, hmacSecret)

	addr := ":" + opts.Port
	// Run server in a goroutine
	go func() {
		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			log.WithFields(logrus.Fields{
				"address": addr,
				"error":   err,
			}).Fatal("Server failed to start")
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		log.WithField("error", err).Error("Graceful shutdown failed")
	}
}
