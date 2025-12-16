package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"ship-status-dash/pkg/types"
)

// Options contains command-line configuration options for the component monitor.
type Options struct {
	ConfigPath   string
	DashboardURL string
	Name         string
	// KubeconfigDir is the path to a directory containing kubeconfig files for different clusters.
	// Each file should be named after the cluster with a ".config" suffix (e.g., "app.ci.config" for the app.ci cluster).
	// When this is set, prometheusLocation in the config must be a cluster name, not a URL.
	KubeconfigDir string
}

// NewOptions parses command-line flags and returns a new Options instance.
func NewOptions() *Options {
	opts := &Options{}

	flag.StringVar(&opts.ConfigPath, "config-path", "", "Path to component monitor config file")
	flag.StringVar(&opts.DashboardURL, "dashboard-url", "http://localhost:8080", "Dashboard API base URL")
	flag.StringVar(&opts.Name, "name", "", "Name of the component monitor")
	flag.StringVar(&opts.KubeconfigDir, "kubeconfig-dir", "", "Path to directory containing kubeconfig files for different clusters (each file named after the cluster)")
	flag.Parse()

	return opts
}

// Validate checks that all required options are provided and valid.
func (o *Options) Validate() error {
	if o.ConfigPath == "" {
		return errors.New("config path is required (use --config-path flag)")
	}

	if _, err := os.Stat(o.ConfigPath); os.IsNotExist(err) {
		return errors.New("config file does not exist: " + o.ConfigPath)
	}

	if o.Name == "" {
		return errors.New("name is required (use --name flag)")
	}

	return nil
}

func loadAndValidateComponentsAndFrequency(log *logrus.Logger, configPath string) ([]types.MonitoringComponent, time.Duration) {
	log.Infof("Loading config from %s", configPath)

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		log.WithFields(logrus.Fields{
			"config_path": configPath,
			"error":       err,
		}).Fatal("Failed to read config file")
	}

	var config types.ComponentMonitorConfig
	if err := yaml.Unmarshal(configFile, &config); err != nil {
		log.WithFields(logrus.Fields{
			"config_path": configPath,
			"error":       err,
		}).Fatal("Failed to parse config file")
	}

	frequency, err := time.ParseDuration(config.Frequency)
	if err != nil {
		log.WithFields(logrus.Fields{
			"frequency": config.Frequency,
			"error":     err,
		}).Fatal("Failed to parse frequency")
	}
	log.Infof("Probing Frequency configured to: %s", frequency)

	for _, component := range config.Components {
		if component.HTTPMonitor != nil {
			retryAfter, err := time.ParseDuration(component.HTTPMonitor.RetryAfter)
			if err != nil {
				log.WithField("error", err).Fatal("Failed to parse retry after duration")
			}
			if retryAfter > frequency {
				log.WithFields(logrus.Fields{
					"component":     component.ComponentSlug,
					"sub_component": component.SubComponentSlug,
					"retry_after":   component.HTTPMonitor.RetryAfter,
					"frequency":     frequency,
				}).Fatal("Retry after duration is greater than frequency")
			}
		}
	}

	setDefaultStepValues(&config)

	log.Infof("Loaded configuration with %d components", len(config.Components))
	return config.Components, frequency
}

func main() {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	opts := NewOptions()

	if err := opts.Validate(); err != nil {
		log.WithField("error", err).Fatal("Invalid command-line options")
	}

	components, frequency := loadAndValidateComponentsAndFrequency(log, opts.ConfigPath)

	// Validate Prometheus configuration
	if err := validatePrometheusConfiguration(components, opts.KubeconfigDir); err != nil {
		log.WithField("error", err).Fatal("Invalid prometheus location configuration")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Received interrupt signal, shutting down...")
		cancel()
	}()

	prometheusClients, err := createPrometheusClients(components, opts.KubeconfigDir)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to create prometheus clients")
	}

	var probers []Prober
	for _, component := range components {
		componentLogger := log.WithFields(logrus.Fields{
			"component":     component.ComponentSlug,
			"sub_component": component.SubComponentSlug,
		})
		componentLogger.Info("Configuring component monitor probe")
		if component.HTTPMonitor != nil {
			retryAfter, err := time.ParseDuration(component.HTTPMonitor.RetryAfter)
			if err != nil {
				componentLogger.WithField("error", err).Fatal("Failed to parse retry after duration")
			}
			prober := NewHTTPProber(component.ComponentSlug, component.SubComponentSlug, component.HTTPMonitor.URL, component.HTTPMonitor.Code, retryAfter)
			componentLogger.Info("Added HTTP prober for component")
			probers = append(probers, prober)
		}
		if component.PrometheusMonitor != nil {
			prometheusProber := NewPrometheusProber(component.ComponentSlug, component.SubComponentSlug, prometheusClients[component.PrometheusMonitor.PrometheusLocation], component.PrometheusMonitor.Queries)
			componentLogger.Info("Added Prometheus prober for component")
			probers = append(probers, prometheusProber)
		}
	}

	if len(probers) == 0 {
		log.Warn("No probers configured, exiting")
		return
	}

	orchestrator := NewProbeOrchestrator(probers, frequency, opts.DashboardURL, opts.Name, log)
	orchestrator.Run(ctx)
}
